package audit

import "context"

// ContentSink 是审计数据的消费者接口。
// 实现方（service.LogContentSink）负责异步落库；调用方只持有此接口，不持有实现。
//
// 分层约束（BR-004 / F-34）：本包只能 import common、relaykit/dto、relaykit/types，
// 禁止 import model 或 relay/common，否则与 relay/common → audit → model → relay/common 成环。
type ContentSink interface {
	OnInput(snap InputSnapshot)
	OnOutput(snap OutputSnapshot)
	OnSettled(snap UsageSnapshot, ctx context.Context)
}

// Fidelity 表示一条审计记录的采集保真度。
const (
	// FidelityStructured 结构化：segments 由格式 walker 从 DTO 解析得到。
	FidelityStructured = "structured"
	// FidelityOpaque 不透明：无法结构化解析，只保留整体摘要（hash / bytes）。
	FidelityOpaque = "opaque"
	// FidelityMetaOnly 仅元数据：不采集任何正文。
	FidelityMetaOnly = "meta_only"
)

// SegmentMode 表示单个 segment 的留存方式（BR-008）。
const (
	ModeFull    = "full"
	ModePreview = "preview"
	ModeDerive  = "derive"
	ModeDrop    = "drop"
	ModeOmitted = "omitted"
)

// SegmentKind 表示 segment 的角色分类。
const (
	KindSystem     = "system"
	KindUser       = "user"
	KindAssistant  = "assistant"
	KindToolCall   = "tool_call"
	KindToolResult = "tool_result"
	KindImage      = "image"
	KindAudio      = "audio"
)

// InputSnapshot 是 OnInput 的入参快照：一次请求的输入侧内容。
type InputSnapshot struct {
	RequestId string
	ModelName string
	Segments  []Segment
	Fidelity  string
}

// OutputSnapshot 是 OnOutput 的入参快照：一次请求的输出侧内容。
type OutputSnapshot struct {
	RequestId string
	Segments  []Segment
}

// UsageSnapshot 是 OnSettled 的入参快照：结算时的用量与归属信息。
type UsageSnapshot struct {
	RequestId        string
	UserId           int
	ChannelId        int
	ModelName        string
	PromptTokens     int
	CompletionTokens int
	Quota            int
	WLVersion        int
}

// Segment 是审计正文的一个分片。
type Segment struct {
	Kind      string        `json:"kind"`
	Idx       int           `json:"idx"`
	Text      string        `json:"text,omitempty"`
	Bytes     int           `json:"bytes"`
	Mode      string        `json:"mode"`
	Truncated bool          `json:"truncated,omitempty"`
	SHA256    string        `json:"sha256,omitempty"`
	Derived   *DerivedFacts `json:"derived,omitempty"`
	Reason    string        `json:"reason,omitempty"`
}

// DerivedFacts 是从正文中提取的派生事实（BR-007：先 derive 再 drop）。
type DerivedFacts struct {
	URLs     []string `json:"urls,omitempty"`
	Domains  []string `json:"domains,omitempty"`
	Tools    []string `json:"tools,omitempty"`
	ArgsKeys []string `json:"args_keys,omitempty"`
	Chars    int      `json:"chars,omitempty"`
}

// SegmentConfig 控制分段构建策略。
type SegmentConfig struct {
	// PerRequestMaxBytes 单请求最大采集字节数（BR-009 降级依据）。
	PerRequestMaxBytes int
}

// HitFlag 是一条 watchlist 命中记录（BR-012：存 rule_id + pattern 快照，
// 规则被改/删后历史记录含义不变）。
type HitFlag struct {
	RuleId         uint   `json:"rule_id"`
	PatternSnapshot string `json:"pattern_snapshot"`
	Kind           string `json:"kind"`
	Severity       string `json:"severity"`
	SegIdx         int    `json:"seg_idx"`
}
