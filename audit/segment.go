package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// 各 kind 默认留存策略（BR-008）：
// system=preview/512B, user=full/16KB, assistant=full/16KB,
// tool_call=derive/1KB, tool_result=drop, image=omitted, audio=omitted.
const (
	DefaultSystemPreviewBytes    = 512
	DefaultUserFullBytes         = 16 * 1024
	DefaultAssistantFullBytes    = 16 * 1024
	DefaultToolCallDeriveBytes   = 1024
	DefaultToolDefPreviewBytes   = 1024
	DefaultPreviewBytes          = 512
	MaxOpaqueReadBytes           = 1024 * 1024
)

type kindPolicy struct {
	mode  string
	limit int
}

var defaultKindPolicy = map[string]kindPolicy{
	KindSystem:     {ModePreview, DefaultSystemPreviewBytes},
	KindUser:       {ModeFull, DefaultUserFullBytes},
	KindAssistant:  {ModeFull, DefaultAssistantFullBytes},
	KindToolCall:   {ModeDerive, DefaultToolCallDeriveBytes},
	KindToolResult: {ModeDrop, 0},
	KindToolDef:    {ModePreview, DefaultToolDefPreviewBytes},
	KindImage:      {ModeOmitted, 0},
	KindAudio:      {ModeOmitted, 0},
}

// downgradeOrder 是超预算时的降级优先级（BR-009）：
// tool_result 先砍，user 最后砍。值越小越先被降级。
var downgradeOrder = []string{
	KindToolResult,
	KindToolDef,
	KindToolCall,
	KindSystem,
	KindAssistant,
	KindUser,
}

// BuildOpenAISegments 将 OpenAI 格式的 Message 列表转为 []Segment。
// 依赖 msg.ParseContent()（真实定义 relaykit/dto/openai_request.go L543），
// 不可用 GetTokenCountMeta 的 CombineText（L202 已丢失角色信息，F-23）。
func BuildOpenAISegments(msgs []dto.Message, cfg SegmentConfig) []Segment {
	if len(msgs) == 0 {
		return nil
	}
	segs := make([]Segment, 0, len(msgs))
	for i := range msgs {
		msg := &msgs[i]
		segs = append(segs, buildOpenAIMessageSegments(msg, i)...)
	}
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

func buildOpenAIMessageSegments(msg *dto.Message, idx int) []Segment {
	role := strings.ToLower(msg.Role)
	kind := kindByRole(role)
	var segs []Segment
	for _, mc := range msg.ParseContent() {
		switch mc.Type {
		case dto.ContentTypeText:
			segs = append(segs, makeTextSegment(kind, mc.Text, idx))
		case dto.ContentTypeImageURL:
			segs = append(segs, makeOmittedSegment(KindImage, idx, "image_content"))
		case dto.ContentTypeInputAudio:
			segs = append(segs, makeOmittedSegment(KindAudio, idx, "audio_content"))
		case dto.ContentTypeFile:
			segs = append(segs, makeOmittedSegment(KindImage, idx, "file_content"))
		case dto.ContentTypeVideoUrl:
			segs = append(segs, makeOmittedSegment(KindImage, idx, "video_content"))
		default:
			segs = append(segs, makeOmittedSegment(KindImage, idx, "non_text_content"))
		}
	}

	// assistant 消息可携带 tool_calls：提取工具名与参数 key（BR-007）。
	if len(msg.ToolCalls) > 0 {
		var calls []dto.ToolCallRequest
		if err := common.Unmarshal(msg.ToolCalls, &calls); err == nil {
			for _, call := range calls {
				segs = append(segs, makeToolCallSegment(call, idx))
			}
		}
	}
	return segs
}

func kindByRole(role string) string {
	switch role {
	case "system":
		return KindSystem
	case "assistant":
		return KindAssistant
	case "tool":
		return KindToolResult
	default:
		return KindUser
	}
}

func makeTextSegment(kind, text string, idx int) Segment {
	policy := defaultKindPolicy[kind]
	if policy.limit <= 0 {
		return makeDropSegment(kind, text, idx, "no_text_retention")
	}
	seg := Segment{
		Kind:   kind,
		Idx:    idx,
		Bytes:  len(text),
		Mode:   policy.mode,
		SHA256: sha256Hex([]byte(text)),
	}
	// BR-007：先 derive 再决定留存；full/preview 也保留派生事实供 domain watchlist 匹配。
	seg.Derived = deriveFacts(text)
	if len(text) > policy.limit {
		seg.Text = truncateUTF8(text, policy.limit)
		seg.Truncated = true
	} else {
		seg.Text = text
	}
	return seg
}

func makeOmittedSegment(kind string, idx int, reason string) Segment {
	return Segment{
		Kind:   kind,
		Idx:    idx,
		Mode:   ModeOmitted,
		Reason: reason,
	}
}

func makeDropSegment(kind, text string, idx int, reason string) Segment {
	derived := deriveFacts(text)
	seg := Segment{
		Kind:     kind,
		Idx:      idx,
		Bytes:    len(text),
		Mode:     ModeDrop,
		SHA256:   sha256Hex([]byte(text)),
		Derived:  derived,
		Reason:   reason,
		ScanText: text,
	}
	if derived == nil {
		seg.Derived = &DerivedFacts{Chars: len([]rune(text))}
	}
	return seg
}

func makeToolCallSegment(call dto.ToolCallRequest, idx int) Segment {
	name := call.Function.Name
	var argsKeys []string
	if call.Function.Arguments != "" {
		var args map[string]any
		if err := common.Unmarshal([]byte(call.Function.Arguments), &args); err == nil {
			for k := range args {
				argsKeys = append(argsKeys, k)
			}
		}
	}
	policy := defaultKindPolicy[KindToolCall]
	derived := &DerivedFacts{Tools: []string{name}, ArgsKeys: argsKeys}
	// BR-103：从参数全文 derive URLs/Domains，让 domain 档也能命中 tool_call。
	if argsText := call.Function.Arguments; argsText != "" {
		if derivedFromText := deriveFacts(argsText); derivedFromText != nil {
			derived.URLs = append(derived.URLs, derivedFromText.URLs...)
			derived.Domains = append(derived.Domains, derivedFromText.Domains...)
		}
	}
	seg := Segment{
		Kind:     KindToolCall,
		Idx:      idx,
		Bytes:    len(call.Function.Arguments) + len(name),
		Mode:     policy.mode,
		SHA256:   sha256Hex([]byte(call.Function.Arguments)),
		Derived:  derived,
		ScanText: call.Function.Arguments,
	}
	if policy.limit > 0 && len(call.Function.Arguments) > policy.limit {
		seg.Text = truncateUTF8(call.Function.Arguments, policy.limit)
		seg.Truncated = true
	} else if policy.mode == ModeDerive {
		seg.Text = call.Function.Arguments
	}
	return seg
}

// BuildOpaqueSegment 是结构化解析失败时的兜底（fidelity=opaque）。
// 按 ASM-003 以 preview 留存：保留前 512B 文本 + 派生事实 + hash。
func BuildOpaqueSegment(body []byte, cfg SegmentConfig) []Segment {
	if len(body) == 0 {
		return nil
	}
	text := string(body)
	seg := Segment{
		Kind:   KindUser,
		Bytes:  len(body),
		Mode:   ModePreview,
		SHA256: sha256Hex(body),
	}
	if len(text) > DefaultPreviewBytes {
		seg.Text = truncateUTF8(text, DefaultPreviewBytes)
		seg.Truncated = true
	} else {
		seg.Text = text
	}
	seg.Derived = deriveFacts(text)
	return []Segment{seg}
}

// BuildMetaSegment 返回空 segments（fidelity=meta_only）。
func BuildMetaSegment() []Segment {
	return nil
}

// BuildAssistantOutputSegment 构建 assistant 输出段（Phase 2 OnOutput）。
// 段上限 = min(assistant 默认 16KB, per_request_max_bytes)；超限降为 preview 并标记截断。
func BuildAssistantOutputSegment(text string, maxBytes int) Segment {
	limit := DefaultAssistantFullBytes
	if maxBytes > 0 && maxBytes < limit {
		limit = maxBytes
	}
	seg := Segment{
		Kind:    KindAssistant,
		Bytes:   len(text),
		Mode:    ModeFull,
		SHA256:  sha256Hex([]byte(text)),
		Derived: deriveFacts(text),
	}
	if len(text) > limit {
		seg.Mode = ModePreview
		seg.Text = truncateUTF8(text, limit)
		seg.Truncated = true
	} else {
		seg.Text = text
	}
	return seg
}

// BuildOutputToolCallSegments 将响应侧的 tool_calls 转为独立 tool_call 输出段（BR-107）。
// 非流式与流式共用：调用方传入已解析的 ToolCallRequest 切片。
func BuildOutputToolCallSegments(toolCalls []dto.ToolCallRequest, maxBytes int) []Segment {
	if len(toolCalls) == 0 {
		return nil
	}
	segs := make([]Segment, 0, len(toolCalls))
	for i, tc := range toolCalls {
		segs = append(segs, makeToolCallSegment(tc, i))
	}
	if maxBytes > 0 {
		segs = applyBudget(segs, maxBytes)
	}
	return segs
}

// Claude 消息内容类型
const (
	claudeTextType      = "text"
	claudeImageType     = "image"
	claudeToolUseType   = "tool_use"
	claudeToolResultType = "tool_result"
	claudeThinkingType  = "thinking"
)

// BuildClaudeSegments 将 Claude 格式请求转为 []Segment（Phase 3）。
// 依赖 req.ParseSystem() 与 msg.ParseContent()（relaykit/dto/claude.go）。
func BuildClaudeSegments(req *dto.ClaudeRequest, cfg SegmentConfig) []Segment {
	if req == nil {
		return nil
	}
	var segs []Segment
	idx := 0

	// system：可能是字符串或结构化数组（ParseSystem 只处理数组）。
	if req.IsStringSystem() {
		if text := req.GetStringSystem(); text != "" {
			segs = append(segs, makeTextSegment(KindSystem, text, idx))
			idx++
		}
	}
	for _, sys := range req.ParseSystem() {
		if sys.Type == claudeTextType {
			segs = append(segs, makeTextSegment(KindSystem, sys.GetText(), idx))
			idx++
		}
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		content, err := msg.ParseContent()
		if err != nil {
			if text := msg.GetStringContent(); text != "" {
				segs = append(segs, makeTextSegment(kindByRole(msg.Role), text, idx))
				idx++
			}
			continue
		}
		for _, mc := range content {
			switch mc.Type {
			case claudeTextType:
				segs = append(segs, makeTextSegment(kindByRole(msg.Role), mc.GetText(), idx))
			case claudeImageType:
				segs = append(segs, makeOmittedSegment(KindImage, idx, "image_content"))
			case claudeToolResultType:
				// tool_result 块按 tool_result 处理（即使嵌在 user 消息内，BR-008）。
				segs = append(segs, makeDropSegment(KindToolResult, mc.GetStringContent(), idx, "tool_result"))
			case claudeToolUseType:
				segs = append(segs, makeClaudeToolUseSegment(mc, idx))
			case claudeThinkingType:
				segs = append(segs, makeTextSegment(KindAssistant, mc.GetText(), idx))
			default:
				if text := mc.GetText(); text != "" {
					segs = append(segs, makeTextSegment(kindByRole(msg.Role), text, idx))
				}
			}
			idx++
		}
	}
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

func makeClaudeToolUseSegment(mc dto.ClaudeMediaMessage, idx int) Segment {
	derived := &DerivedFacts{}
	if mc.Name != "" {
		derived.Tools = []string{mc.Name}
	}
	var argsText string
	if mc.Input != nil {
		if b, err := common.Marshal(mc.Input); err == nil {
			argsText = string(b)
			var m map[string]any
			if common.Unmarshal(b, &m) == nil {
				for k := range m {
					derived.ArgsKeys = append(derived.ArgsKeys, k)
				}
			}
		}
	}
	// BR-103：从参数全文 derive URLs/Domains，让 domain 档也能命中 tool_call。
	if argsText != "" {
		if derivedFromText := deriveFacts(argsText); derivedFromText != nil {
			derived.URLs = append(derived.URLs, derivedFromText.URLs...)
			derived.Domains = append(derived.Domains, derivedFromText.Domains...)
		}
	}
	policy := defaultKindPolicy[KindToolCall]
	text := argsText
	truncated := false
	if policy.limit > 0 && len(argsText) > policy.limit {
		text = truncateUTF8(argsText, policy.limit)
		truncated = true
	}
	return Segment{
		Kind:      KindToolCall,
		Idx:       idx,
		Bytes:     len(argsText),
		Mode:      ModeDerive,
		Truncated: truncated,
		Text:      text,
		Derived:   derived,
		ScanText:  argsText,
	}
}

// BuildGeminiSegments 将 Gemini 格式请求转为 []Segment（Phase 3）。
func BuildGeminiSegments(req *dto.GeminiChatRequest, cfg SegmentConfig) []Segment {
	if req == nil {
		return nil
	}
	var segs []Segment
	idx := 0

	if sys := req.SystemInstructions; sys != nil {
		for _, part := range sys.Parts {
			if part.Text != "" {
				segs = append(segs, makeTextSegment(KindSystem, part.Text, idx))
				idx++
			}
		}
	}

	for ci := range req.Contents {
		content := &req.Contents[ci]
		kind := KindUser
		switch content.Role {
		case "model":
			kind = KindAssistant
		case "function":
			kind = KindToolResult
		}
		for _, part := range content.Parts {
			switch {
			case part.Text != "":
				segs = append(segs, makeTextSegment(kind, part.Text, idx))
			case part.InlineData != nil:
				kind2 := KindImage
				if strings.HasPrefix(part.InlineData.MimeType, "audio/") {
					kind2 = KindAudio
				}
				segs = append(segs, makeOmittedSegment(kind2, idx, "inline_data"))
			case part.FunctionCall != nil:
				segs = append(segs, makeGeminiFunctionCallSegment(part.FunctionCall, idx))
			case part.FunctionResponse != nil:
				segs = append(segs, makeGeminiFunctionResponseSegment(part.FunctionResponse, idx))
			}
			idx++
		}
	}
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

func makeGeminiFunctionCallSegment(fc *dto.FunctionCall, idx int) Segment {
	derived := &DerivedFacts{}
	if fc.FunctionName != "" {
		derived.Tools = []string{fc.FunctionName}
	}
	var argsText string
	if fc.Arguments != nil {
		if b, err := common.Marshal(fc.Arguments); err == nil {
			argsText = string(b)
			var m map[string]any
			if common.Unmarshal(b, &m) == nil {
				for k := range m {
					derived.ArgsKeys = append(derived.ArgsKeys, k)
				}
			}
		}
	}
	// BR-103：从参数全文 derive URLs/Domains，让 domain 档也能命中 tool_call。
	if argsText != "" {
		if derivedFromText := deriveFacts(argsText); derivedFromText != nil {
			derived.URLs = append(derived.URLs, derivedFromText.URLs...)
			derived.Domains = append(derived.Domains, derivedFromText.Domains...)
		}
	}
	policy := defaultKindPolicy[KindToolCall]
	text := argsText
	truncated := false
	if policy.limit > 0 && len(argsText) > policy.limit {
		text = truncateUTF8(argsText, policy.limit)
		truncated = true
	}
	return Segment{
		Kind:      KindToolCall,
		Idx:       idx,
		Bytes:     len(argsText),
		Mode:      ModeDerive,
		Truncated: truncated,
		Text:      text,
		Derived:   derived,
		ScanText:  argsText,
	}
}

func makeGeminiFunctionResponseSegment(fr *dto.GeminiFunctionResponse, idx int) Segment {
	text := ""
	if b, err := common.Marshal(fr.Response); err == nil {
		text = string(b)
	}
	return makeDropSegment(KindToolResult, text, idx, "function_response")
}

// makeToolDefSegment 构建工具定义段（BR-105：kind=tool_def, preview/1KB）。
// fullText = name + " " + description + " " + schemaJSON；ScanText 填全文供 watchlist 扫描。
func makeToolDefSegment(kind, name, description, schemaJSON string, idx int) Segment {
	fullText := strings.Join([]string{name, description, schemaJSON}, " ")
	policy := defaultKindPolicy[kind]
	derived := deriveFacts(fullText)
	if derived == nil {
		derived = &DerivedFacts{Chars: len([]rune(fullText))}
	}
	if name != "" {
		derived.Tools = append(derived.Tools, name)
	}
	seg := Segment{
		Kind:     kind,
		Idx:      idx,
		Bytes:    len(fullText),
		Mode:     policy.mode,
		SHA256:   sha256Hex([]byte(fullText)),
		Derived:  derived,
		ScanText: fullText,
	}
	if len(fullText) > policy.limit {
		seg.Text = truncateUTF8(fullText, policy.limit)
		seg.Truncated = true
	} else {
		seg.Text = fullText
	}
	return seg
}

func buildOpenAIToolDefSegments(tools []dto.ToolCallRequest) []Segment {
	if len(tools) == 0 {
		return nil
	}
	segs := make([]Segment, 0, len(tools))
	for i, tool := range tools {
		var schemaJSON string
		if tool.Function.Parameters != nil {
			if b, err := common.Marshal(tool.Function.Parameters); err == nil {
				schemaJSON = string(b)
			}
		}
		segs = append(segs, makeToolDefSegment(KindToolDef, tool.Function.Name, tool.Function.Description, schemaJSON, i))
	}
	return segs
}

func buildClaudeToolDefSegments(tools []any) []Segment {
	normalTools, _ := dto.ProcessTools(tools)
	if len(normalTools) == 0 {
		return nil
	}
	segs := make([]Segment, 0, len(normalTools))
	for i, t := range normalTools {
		var schemaJSON string
		if t.InputSchema != nil {
			if b, err := common.Marshal(t.InputSchema); err == nil {
				schemaJSON = string(b)
			}
		}
		segs = append(segs, makeToolDefSegment(KindToolDef, t.Name, t.Description, schemaJSON, i))
	}
	return segs
}

func buildGeminiToolDefSegments(tools []dto.GeminiChatTool) []Segment {
	if len(tools) == 0 {
		return nil
	}
	var segs []Segment
	idx := 0
	for _, tool := range tools {
		if tool.FunctionDeclarations == nil {
			continue
		}
		if b, err := common.Marshal(tool.FunctionDeclarations); err == nil {
			// FunctionDeclarations 可能是单个对象或数组（上游两种写法）。
			var decls []map[string]any
			if common.Unmarshal(b, &decls) != nil {
				var single map[string]any
				if common.Unmarshal(b, &single) == nil {
					decls = []map[string]any{single}
				}
			}
			for _, decl := range decls {
				name, _ := decl["name"].(string)
				description, _ := decl["description"].(string)
				var schemaJSON string
				if params, ok := decl["parameters"]; ok && params != nil {
					if pb, err := common.Marshal(params); err == nil {
						schemaJSON = string(pb)
					}
				}
				segs = append(segs, makeToolDefSegment(KindToolDef, name, description, schemaJSON, idx))
				idx++
			}
		}
	}
	return segs
}

// BuildOpenAIInputSegments 是 OpenAI 格式的完整输入段构建入口（BR-105）：
// 消息段（含 tool_call/tool_result）+ tools 定义段（kind=tool_def）。
func BuildOpenAIInputSegments(req *dto.GeneralOpenAIRequest, cfg SegmentConfig) []Segment {
	if req == nil {
		return nil
	}
	segs := BuildOpenAISegments(req.Messages, cfg)
	segs = append(segs, buildOpenAIToolDefSegments(req.Tools)...)
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

// BuildClaudeInputSegments 是 Claude 格式的完整输入段构建入口（BR-105）。
func BuildClaudeInputSegments(req *dto.ClaudeRequest, cfg SegmentConfig) []Segment {
	if req == nil {
		return nil
	}
	segs := BuildClaudeSegments(req, cfg)
	segs = append(segs, buildClaudeToolDefSegments(req.GetTools())...)
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

// BuildGeminiInputSegments 是 Gemini 格式的完整输入段构建入口（BR-105）。
func BuildGeminiInputSegments(req *dto.GeminiChatRequest, cfg SegmentConfig) []Segment {
	if req == nil {
		return nil
	}
	segs := BuildGeminiSegments(req, cfg)
	segs = append(segs, buildGeminiToolDefSegments(req.GetTools())...)
	return applyBudget(segs, cfg.PerRequestMaxBytes)
}

// applyBudget 执行 BR-009：当总保留字节数超过 per_request_max_bytes 时，
// 按 downgradeOrder 优先级逐段降级，直到满足预算。user 最后被砍。
func applyBudget(segs []Segment, maxBytes int) []Segment {
	if maxBytes <= 0 || len(segs) == 0 {
		return segs
	}
	total := segsTotalBytes(segs)
	if total <= maxBytes {
		return segs
	}
	for total > maxBytes {
		progress := false
		for _, kind := range downgradeOrder {
			for i := range segs {
				if segs[i].Kind != kind || !canDowngrade(segs[i].Mode) {
					continue
				}
				before := len(segs[i].Text)
				segs[i] = downgradeSegment(segs[i])
				total = total - before + len(segs[i].Text)
				progress = true
				if total <= maxBytes {
					break
				}
			}
			if total <= maxBytes {
				break
			}
		}
		if !progress {
			break
		}
	}
	return segs
}

// segsTotalBytes 统计实际保留的文本字节数（drop/omitted 不占预算）。
func segsTotalBytes(segs []Segment) int {
	total := 0
	for _, s := range segs {
		total += len(s.Text)
	}
	return total
}

func canDowngrade(mode string) bool {
	return mode == ModeFull || mode == ModePreview || mode == ModeDerive
}

// downgradeSegment 将 segment 降级一档：full→preview→drop；derive→drop。
// 已经是 drop/omitted 的 segment 原样返回。
func downgradeSegment(s Segment) Segment {
	switch s.Mode {
	case ModeFull:
		s.Mode = ModePreview
		s.Text = truncateUTF8(s.Text, DefaultPreviewBytes)
		s.Truncated = true
	case ModePreview:
		derived := s.Derived
		if derived == nil {
			derived = deriveFacts(s.Text)
		}
		s.Text = ""
		s.Mode = ModeDrop
		s.Truncated = true
		s.Derived = derived
		s.Reason = "budget"
	case ModeDerive:
		s.Text = ""
		s.Mode = ModeDrop
		s.Truncated = true
		s.Reason = "budget"
	}
	return s
}

// deriveFacts 从文本提取派生事实（BR-007：先 derive 再 drop）。
func deriveFacts(text string) *DerivedFacts {
	if text == "" {
		return nil
	}
	urls := extractURLs(text)
	domains := extractDomains(urls)
	facts := &DerivedFacts{Chars: len([]rune(text))}
	if len(urls) > 0 {
		facts.URLs = urls
	}
	if len(domains) > 0 {
		facts.Domains = domains
	}
	if len(urls) == 0 && len(domains) == 0 {
		return facts
	}
	return facts
}

var urlRe = regexp.MustCompile(`https?://[^\s"'<>\]\)]+`)

func extractURLs(text string) []string {
	matches := urlRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		u := strings.TrimRight(m, ".,;:!?")
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

func extractDomains(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(urls))
	var out []string
	for _, u := range urls {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Host == "" {
			continue
		}
		host := parsed.Hostname()
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return strings.ToValidUTF8(s[:maxBytes], "")
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
