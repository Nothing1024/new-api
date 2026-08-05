package service

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/audit"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

var severityRank = map[string]int{"low": 1, "medium": 2, "high": 3}

// MaxSeverity 取 flags 中的最高 severity（空返回 ""）。
func MaxSeverity(flags []audit.HitFlag) string {
	best := ""
	bestRank := 0
	for _, f := range flags {
		if rank := severityRank[f.Severity]; rank > bestRank {
			best = f.Severity
			bestRank = rank
		}
	}
	return best
}

type compiledRegex struct {
	rule model.AuditWatchlistRule
	re   *regexp.Regexp
}

// ScanSegments 对 segments 执行 watchlist 三档扫描（domain/keyword/regex），返回命中 flag 列表。
// BR-007：domain 档从 Derived.Domains 匹配；keyword/regex 从保留文本匹配。
func ScanSegments(segs []audit.Segment, rules []model.AuditWatchlistRule) []audit.HitFlag {
	if len(segs) == 0 || len(rules) == 0 {
		return nil
	}
	var flags []audit.HitFlag
	domainRuleMap := make(map[string][]model.AuditWatchlistRule)
	keywordRules := make([]string, 0)
	keywordRuleMap := make(map[string][]model.AuditWatchlistRule)
	var regexRules []compiledRegex

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Kind {
		case model.WatchlistKindDomain:
			d := strings.ToLower(strings.TrimSpace(r.Pattern))
			if d != "" {
				domainRuleMap[d] = append(domainRuleMap[d], r)
			}
		case model.WatchlistKindKeyword:
			w := strings.ToLower(strings.TrimSpace(r.Pattern))
			if w != "" {
				keywordRules = append(keywordRules, w)
				keywordRuleMap[w] = append(keywordRuleMap[w], r)
			}
		case model.WatchlistKindRegex:
			if re, err := regexp.Compile(r.Pattern); err == nil {
				regexRules = append(regexRules, compiledRegex{rule: r, re: re})
			} else {
				logger.LogWarn(context.Background(), "audit watchlist regex compile failed: "+err.Error())
			}
		}
	}

	for si, seg := range segs {
		// domain 档：derived facts（先 derive 再 drop，BR-007）
		if seg.Derived != nil {
			for _, d := range seg.Derived.Domains {
				if matches, ok := domainRuleMap[strings.ToLower(d)]; ok {
					for _, r := range matches {
						flags = append(flags, audit.HitFlag{RuleId: r.Id, PatternSnapshot: r.Pattern, Kind: r.Kind, Severity: r.Severity, SegIdx: si})
					}
				}
			}
		}
		// keyword 档：AC 自动机
		if len(keywordRules) > 0 && seg.Text != "" {
			if ok, words := AcSearch(strings.ToLower(seg.Text), keywordRules, false); ok {
				for _, w := range words {
					for _, r := range keywordRuleMap[w] {
						flags = append(flags, audit.HitFlag{RuleId: r.Id, PatternSnapshot: r.Pattern, Kind: r.Kind, Severity: r.Severity, SegIdx: si})
					}
				}
			}
		}
		// regex 档（BR-010：≤8 条 enabled）
		if len(regexRules) > 0 && seg.Text != "" {
			for _, cr := range regexRules {
				if cr.re.MatchString(seg.Text) {
					flags = append(flags, audit.HitFlag{RuleId: cr.rule.Id, PatternSnapshot: cr.rule.Pattern, Kind: cr.rule.Kind, Severity: cr.rule.Severity, SegIdx: si})
				}
			}
		}
	}
	return flags
}

// watchlist 规则缓存（version 驱动，BR-011）。
var (
	watchlistCacheMu    sync.RWMutex
	watchlistCacheVer   int
	watchlistCacheRules []model.AuditWatchlistRule
	watchlistCacheAt    time.Time
)

const watchlistCacheTTL = 5 * time.Second

func loadActiveWatchlistRules() []model.AuditWatchlistRule {
	// 先走 TTL 缓存，避免每次扫描都查库（GetWatchlistVersion 是 DB 读）。
	watchlistCacheMu.RLock()
	if watchlistCacheRules != nil && time.Since(watchlistCacheAt) < watchlistCacheTTL {
		rules := watchlistCacheRules
		watchlistCacheMu.RUnlock()
		return rules
	}
	watchlistCacheMu.RUnlock()

	// 缓存过期：读版本，变了才重载（BR-011 版本驱动，5s 内收敛）。
	ver := model.GetWatchlistVersion()
	rules, err := model.ListWatchlistRules(nil, "")
	if err != nil {
		logger.LogWarn(context.Background(), "audit load watchlist rules: "+err.Error())
		return nil
	}
	watchlistCacheMu.Lock()
	watchlistCacheVer = ver
	watchlistCacheRules = rules
	watchlistCacheAt = time.Now()
	watchlistCacheMu.Unlock()
	return rules
}

// scanSegments 供 audit sink 调用：加载规则并扫描，返回 flags 与最高 severity。
func scanSegments(segs []audit.Segment) ([]audit.HitFlag, string) {
	rules := loadActiveWatchlistRules()
	if len(rules) == 0 {
		return nil, ""
	}
	flags := ScanSegments(segs, rules)
	return flags, MaxSeverity(flags)
}

// RescanStatus 是重扫进度的持久化状态。
type RescanStatus struct {
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Status    string `json:"status"` // running / done / error / no_op
	WLVersion int    `json:"wl_version"`
	Message   string `json:"message,omitempty"`
}

const rescanStatusOptionKey = "AuditRescanStatus"

var rescanRunningMu sync.Mutex

// TriggerRescan 启动后台重扫 goroutine；已有重扫运行时返回 false。
func TriggerRescan(ctx context.Context, wlVersion int) (bool, error) {
	if !common.IsMasterNode {
		return false, nil
	}
	rescanRunningMu.Lock()
	defer rescanRunningMu.Unlock()
	var cur RescanStatus
	_ = GetRescanStatus(ctx, &cur)
	if cur.Status == "running" {
		return false, nil
	}
	_ = saveRescanStatus(RescanStatus{Status: "running", WLVersion: wlVersion})
	go runRescan(ctx, wlVersion)
	return true, nil
}

func runRescan(ctx context.Context, wlVersion int) {
	ttlDays := common.AuditContentTTLDays
	cutoff := common.GetTimestamp() - int64(ttlDays)*86400
	total, err := model.CountLogContentsForRescan(wlVersion, cutoff)
	if err != nil {
		logger.LogWarn(ctx, "audit rescan count: "+err.Error())
		_ = saveRescanStatus(RescanStatus{Status: "error", Message: err.Error()})
		return
	}
	if total == 0 {
		_ = saveRescanStatus(RescanStatus{Status: "no_op", Total: 0, WLVersion: wlVersion, Message: "nothing to rescan"})
		logger.LogInfo(ctx, "audit rescan: nothing to rescan")
		return
	}
	rules := loadActiveWatchlistRules()
	processed := 0
	const batchSize = 500
	for offset := 0; ; offset += batchSize {
		rows, err := model.ListLogContentsForRescan(wlVersion, cutoff, batchSize, offset)
		if err != nil {
			logger.LogWarn(ctx, "audit rescan batch: "+err.Error())
			_ = saveRescanStatus(RescanStatus{Status: "error", Message: err.Error(), Processed: processed, Total: int(total)})
			return
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			var segs []audit.Segment
			if row.Segments != "" {
				_ = common.UnmarshalJsonStr(row.Segments, &segs)
			}
			flags := ScanSegments(segs, rules)
			flagsJSON := "[]"
			if len(flags) > 0 {
				if b, err := common.Marshal(flags); err == nil {
					flagsJSON = string(b)
				}
			}
			if err := model.UpdateLogContentFlags(row.RequestId, MaxSeverity(flags), len(flags), flagsJSON, wlVersion); err != nil {
				logger.LogWarn(ctx, "audit rescan update "+row.RequestId+": "+err.Error())
			}
			processed++
		}
		_ = saveRescanStatus(RescanStatus{Processed: processed, Total: int(total), Status: "running", WLVersion: wlVersion})
		time.Sleep(100 * time.Millisecond) // 限速（spec：500/批 + 100ms sleep）
		if len(rows) < batchSize {
			break
		}
	}
	_ = saveRescanStatus(RescanStatus{Processed: processed, Total: int(total), Status: "done", WLVersion: wlVersion})
	logger.LogInfo(ctx, "audit rescan done: processed="+strconv.Itoa(processed))
}

// GetRescanStatus 读取当前重扫进度。
func GetRescanStatus(ctx context.Context, out *RescanStatus) error {
	common.OptionMapRWMutex.RLock()
	v := common.OptionMap[rescanStatusOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if v == "" {
		return nil
	}
	return common.UnmarshalJsonStr(v, out)
}

func saveRescanStatus(st RescanStatus) error {
	b, err := common.Marshal(st)
	if err != nil {
		return err
	}
	common.OptionMapRWMutex.Lock()
	common.OptionMap[rescanStatusOptionKey] = string(b)
	common.OptionMapRWMutex.Unlock()
	return nil
}
