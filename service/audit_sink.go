package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/audit"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	auditSinkChannelSize  = 1024
	auditSinkPendingTTL   = 60 * time.Second
	auditSinkFlushedTTL   = 2 * time.Minute
	auditPointerRetries   = 10
	auditPointerRetryGap  = 20 * time.Millisecond
)

type auditEventKind int

const (
	auditEventInput auditEventKind = iota
	auditEventOutput
	auditEventSettled
)

type auditEvent struct {
	kind   auditEventKind
	reqId  string
	input  *audit.InputSnapshot
	output *audit.OutputSnapshot
	usage  *audit.UsageSnapshot
	ctx    context.Context
}

type pendingRecord struct {
	requestId string
	modelName string
	segments  []audit.Segment
	fidelity  string
	usage     *audit.UsageSnapshot
	ctx       context.Context
	createdAt time.Time
}

// LogContentSink 是 audit.ContentSink 的实现（唯一同时 import audit + model 的层）。
// 全部事件经 buffered channel 异步投递；channel 满时立即 drop（BR-006），绝不影响 relay 主链路。
// 所有 map 访问都在单个 worker goroutine 内进行，无需额外加锁。
type LogContentSink struct {
	ch      chan auditEvent
	dropped atomic.Int64
	pending map[string]*pendingRecord
	// flushed 记录最近已落库的 requestId（OnSettled 先到后，晚到的 OnInput/OnOutput
	// 需要合并进现有行而不是新建记录；本 map 用于区分「首次事件」与「落库后晚到」）。
	flushed map[string]time.Time
}

// NewLogContentSink 构造并启动 worker goroutine。
func NewLogContentSink() *LogContentSink {
	s := &LogContentSink{
		ch:      make(chan auditEvent, auditSinkChannelSize),
		pending: make(map[string]*pendingRecord),
		flushed: make(map[string]time.Time),
	}
	go s.worker()
	return s
}

var (
	auditSinkOnce sync.Once
	auditSink     *LogContentSink
)

// GetAuditSink 返回审计 sink；AuditEnabled=false 时返回 nil（BR-005），
// 调用点做 nil 检查即零开销。
func GetAuditSink() audit.ContentSink {
	if !common.AuditEnabled {
		return nil
	}
	auditSinkOnce.Do(func() {
		auditSink = NewLogContentSink()
	})
	return auditSink
}

// OnInput 异步投递输入快照。
func (s *LogContentSink) OnInput(snap audit.InputSnapshot) {
	s.enqueue(auditEvent{kind: auditEventInput, reqId: snap.RequestId, input: &snap})
}

// OnOutput 异步投递输出快照（Phase 2）。
func (s *LogContentSink) OnOutput(snap audit.OutputSnapshot) {
	s.enqueue(auditEvent{kind: auditEventOutput, reqId: snap.RequestId, output: &snap})
}

// OnSettled 异步投递结算快照。
func (s *LogContentSink) OnSettled(snap audit.UsageSnapshot, ctx context.Context) {
	s.enqueue(auditEvent{kind: auditEventSettled, reqId: snap.RequestId, usage: &snap, ctx: ctx})
}

// DroppedCount 返回因 channel 满而被丢弃的事件数（BR-006 观测）。
func (s *LogContentSink) DroppedCount() int64 {
	return s.dropped.Load()
}

func (s *LogContentSink) enqueue(ev auditEvent) {
	select {
	case s.ch <- ev:
	default:
		n := s.dropped.Add(1)
		if n%100 == 1 {
			logger.LogWarn(context.Background(), fmt.Sprintf("audit sink channel full, dropped %d events", n))
		}
	}
}

func (s *LogContentSink) worker() {
	ticker := time.NewTicker(auditSinkPendingTTL / 2)
	defer ticker.Stop()
	for {
		select {
		case ev := <-s.ch:
			s.handleEvent(ev)
		case <-ticker.C:
			s.sweepStale()
		}
	}
}

func (s *LogContentSink) handleEvent(ev auditEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.LogWarn(ev.ctx, fmt.Sprintf("audit sink panic: %v", r))
		}
	}()
	switch ev.kind {
	case auditEventInput:
		s.applyInput(ev)
	case auditEventOutput:
		s.applyOutput(ev)
	case auditEventSettled:
		s.applySettled(ev)
	}
}

func (s *LogContentSink) applyInput(ev auditEvent) {
	rec, ok := s.pending[ev.reqId]
	if !ok {
		// 已落库（OnSettled 先到）→ 合并进现有行；否则是首次事件，新建记录。
		if s.isRecentlyFlushed(ev.reqId) {
			s.mergeInputToDB(ev.reqId, ev.input)
			return
		}
		rec = s.record(ev.reqId)
	}
	if rec.modelName == "" {
		rec.modelName = ev.input.ModelName
	}
	if len(rec.segments) == 0 {
		rec.segments = ev.input.Segments
		rec.fidelity = ev.input.Fidelity
	} else {
		rec.segments = append(rec.segments, ev.input.Segments...)
	}
}

// mergeInputToDB 在 OnSettled 已落库后收到 OnInput 时，把输入段前置进现有 logs_content 行，
// 并提升 fidelity（meta_only → structured）与 model_name，保留 usage 相关列。
func (s *LogContentSink) mergeInputToDB(reqId string, snap *audit.InputSnapshot) {
	if snap == nil || len(snap.Segments) == 0 {
		return
	}
	lc, err := model.GetLogContent(reqId)
	if err != nil {
		return
	}
	var existing []audit.Segment
	if lc.Segments != "" {
		_ = common.UnmarshalJsonStr(lc.Segments, &existing)
	}
	merged := make([]audit.Segment, 0, len(snap.Segments)+len(existing))
	merged = append(merged, snap.Segments...)
	merged = append(merged, existing...)
	if b, err := common.Marshal(merged); err == nil {
		lc.Segments = string(b)
	}
	if lc.Fidelity == "" || lc.Fidelity == audit.FidelityMetaOnly {
		lc.Fidelity = snap.Fidelity
	}
	if lc.ModelName == "" {
		lc.ModelName = snap.ModelName
	}
	if err := model.UpdateLogContent(lc); err != nil {
		logger.LogWarn(context.Background(), "audit merge input: "+err.Error())
	}
}

func (s *LogContentSink) applyOutput(ev auditEvent) {
	rec, ok := s.pending[ev.reqId]
	if !ok {
		// 已落库（OnSettled 先到）→ 合并进现有行；否则是首次事件，新建记录。
		if s.isRecentlyFlushed(ev.reqId) {
			s.mergeOutputToDB(ev.reqId, ev.output.Segments)
			return
		}
		rec = s.record(ev.reqId)
	}
	rec.segments = append(rec.segments, ev.output.Segments...)
}

func (s *LogContentSink) applySettled(ev auditEvent) {
	rec := s.record(ev.reqId)
	rec.usage = ev.usage
	rec.ctx = ev.ctx
	s.flush(rec)
}

func (s *LogContentSink) record(reqId string) *pendingRecord {
	rec, ok := s.pending[reqId]
	if !ok {
		rec = &pendingRecord{requestId: reqId, createdAt: time.Now()}
		s.pending[reqId] = rec
	}
	return rec
}

// isRecentlyFlushed 判断 requestId 是否刚被落库（OnSettled 先到场景）。
func (s *LogContentSink) isRecentlyFlushed(reqId string) bool {
	_, ok := s.flushed[reqId]
	return ok
}

func (s *LogContentSink) sweepStale() {
	var stale []*pendingRecord
	for reqId, rec := range s.pending {
		if rec.usage == nil && time.Since(rec.createdAt) > auditSinkPendingTTL {
			stale = append(stale, rec)
			_ = reqId
		}
	}
	for _, rec := range stale {
		s.flush(rec)
	}
	// 清理过期的 flushed 标记，避免 map 无限增长。
	for reqId, t := range s.flushed {
		if time.Since(t) > auditSinkFlushedTTL {
			delete(s.flushed, reqId)
		}
	}
}

// flush 将 pending 记录落库：合并 segments → watchlist 扫描 → 写 logs_content →
// 更新 logs.other.admin_info.audit 指针（BR-002）。
func (s *LogContentSink) flush(rec *pendingRecord) {
	defer func() {
		delete(s.pending, rec.requestId)
		s.flushed[rec.requestId] = time.Now()
		if r := recover(); r != nil {
			logger.LogWarn(rec.ctx, fmt.Sprintf("audit flush panic: %v", r))
		}
	}()
	ctx := rec.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	segmentsJSON := "[]"
	if len(rec.segments) > 0 {
		if b, err := common.Marshal(rec.segments); err != nil {
			logger.LogWarn(ctx, "audit marshal segments: "+err.Error())
		} else {
			segmentsJSON = string(b)
		}
	}

	flags, severity := s.scanWatchlist(ctx, rec.segments)
	flagsJSON := "[]"
	if len(flags) > 0 {
		if b, err := common.Marshal(flags); err != nil {
			logger.LogWarn(ctx, "audit marshal flags: "+err.Error())
		} else {
			flagsJSON = string(b)
		}
	}

	fidelity := rec.fidelity
	if fidelity == "" {
		fidelity = audit.FidelityMetaOnly
	}

	lc := &model.LogContent{
		RequestId:   rec.requestId,
		ModelName:   rec.modelName,
		Segments:    segmentsJSON,
		Fidelity:    fidelity,
		Flags:       flagsJSON,
		HitCount:    len(flags),
		HitSeverity: severity,
		CreatedAt:   common.GetTimestamp(),
	}
	if rec.usage != nil {
		lc.UserId = rec.usage.UserId
		lc.ChannelId = rec.usage.ChannelId
		lc.ModelName = rec.usage.ModelName
		lc.PromptTokens = rec.usage.PromptTokens
		lc.CompletionTokens = rec.usage.CompletionTokens
		lc.Quota = rec.usage.Quota
		lc.WLVersion = rec.usage.WLVersion
	}

	if err := model.CreateLogContent(lc); err != nil {
		logger.LogWarn(ctx, "audit write log_content: "+err.Error())
		return
	}

	// BR-002：只有正常结算（usage 非空，意味着 RecordConsumeLog 会写 logs 行）才更新指针。
	if rec.usage != nil {
		s.updateAuditPointer(ctx, rec.requestId, len(flags))
	}
}

// mergeOutputToDB 在 OnSettled 已落库后收到 OnOutput 时，把输出段追加进现有 logs_content 行。
func (s *LogContentSink) mergeOutputToDB(reqId string, segs []audit.Segment) {
	if len(segs) == 0 {
		return
	}
	lc, err := model.GetLogContent(reqId)
	if err != nil {
		return
	}
	var existing []audit.Segment
	if lc.Segments != "" {
		_ = common.UnmarshalJsonStr(lc.Segments, &existing)
	}
	existing = append(existing, segs...)
	if b, err := common.Marshal(existing); err == nil {
		lc.Segments = string(b)
		if err := model.UpdateLogContent(lc); err != nil {
			logger.LogWarn(context.Background(), "audit merge output: "+err.Error())
		}
	}
}

// scanWatchlist 执行 watchlist 命中扫描。P1 骨架返回空；P4（Task 25/26）接入真实扫描。
func (s *LogContentSink) scanWatchlist(ctx context.Context, segs []audit.Segment) ([]audit.HitFlag, string) {
	return scanSegments(segs)
}

// updateAuditPointer 更新 logs.other.admin_info.audit 指针。
// RecordConsumeLog 在 OnSettled 钩子之后同步执行，故这里带重试等待 logs 行出现。
func (s *LogContentSink) updateAuditPointer(ctx context.Context, requestId string, hitCount int) {
	var lastErr error
	for i := 0; i < auditPointerRetries; i++ {
		if lastErr = model.UpdateLogAuditPointer(requestId, hitCount); lastErr == nil {
			return
		}
		time.Sleep(auditPointerRetryGap)
	}
	logger.LogWarn(ctx, fmt.Sprintf("audit update logs.other pointer %s: %v", requestId, lastErr))
}
