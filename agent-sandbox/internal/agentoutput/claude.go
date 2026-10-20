package agentoutput

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ClaudeDecoder struct {
	calls       map[eventKey]Event
	lastText    map[string]string
	lastMessage map[string]string
}

func NewClaudeDecoder() *ClaudeDecoder {
	return &ClaudeDecoder{
		calls: make(map[eventKey]Event), lastText: make(map[string]string), lastMessage: make(map[string]string),
	}
}

func (*ClaudeDecoder) Name() string { return AgentClaude }

type claudeMessage struct {
	Type            claudeMessageType `json:"type"`
	Subtype         string            `json:"subtype"`
	SessionID       string            `json:"session_id"`
	ParentToolUseID string            `json:"parent_tool_use_id"`
	Message         json.RawMessage   `json:"message"`
	Result          string            `json:"result"`
	IsError         bool              `json:"is_error"`
	Errors          []string          `json:"errors"`
	Error           json.RawMessage   `json:"error"`
	Attempt         int               `json:"attempt"`
	MaxRetries      int               `json:"max_retries"`
	RetryDelayMS    int               `json:"retry_delay_ms"`
	Usage           *claudeUsage      `json:"usage"`
	MCPServerErrors []struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"mcp_server_errors"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
}

type claudeBlock struct {
	Type      claudeBlockType `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

func (d *ClaudeDecoder) Decode(line []byte) ([]Event, error) {
	var message claudeMessage
	if err := json.Unmarshal(line, &message); err != nil {
		return nil, err
	}
	if message.Type == "" {
		return nil, errors.New("missing message type")
	}
	// Completed content blocks carry parent IDs, unlike token-level deltas.
	// Consume them once instead of printing both partial and complete content.
	scope := message.SessionID + "\x00" + message.ParentToolUseID
	switch message.Type {
	case claudeStreamEvent, claudeToolProgress:
		return nil, nil
	case claudeAssistant, claudeUser:
		return d.blocks(message, scope)
	case claudeSystem:
		return decodeClaudeSystem(message), nil
	case claudeResult:
		var events []Event
		if message.IsError || strings.HasPrefix(message.Subtype, claudeErrorPrefix) {
			text := strings.Join(message.Errors, "\n")
			if text == "" {
				text = message.Result
			}
			if text == "" {
				text = message.Subtype
			}
			events = append(events, Event{Kind: KindFailure, Scope: scope, Subject: text})
		} else if message.Result != "" && strings.Join(strings.Fields(message.Result), " ") != strings.Join(strings.Fields(d.lastText[scope]), " ") {
			d.lastText[scope] = message.Result
			events = append(events, Event{Kind: KindAgent, Scope: scope, Subject: message.Result})
		}
		if usage := message.Usage; usage != nil {
			// Claude reports cache reads and writes separately from uncached input.
			// Include both so normalized input has the same meaning for every agent.
			input := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
			events = append(events, Event{Kind: KindUsage, Scope: scope, Usage: &TokenUsage{
				InputTokens: input, CachedInputTokens: usage.CacheReadInputTokens,
				OutputTokens: usage.OutputTokens, TotalTokens: input + usage.OutputTokens,
			}})
		}
		return events, nil
	default:
		return []Event{{Kind: KindNotice, Subject: "Unsupported " + AgentClaude + " message: " + string(message.Type)}}, nil
	}
}

func (d *ClaudeDecoder) blocks(message claudeMessage, scope string) ([]Event, error) {
	var body struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(message.Message, &body); err != nil {
		return nil, err
	}
	if message.Type == claudeAssistant && (body.ID == "" || body.ID != d.lastMessage[scope]) {
		d.lastText[scope] = ""
		d.lastMessage[scope] = body.ID
	}
	// User text can be a string and includes prompts forwarded to subagents.
	// Only user tool results belong in an execution audit.
	var text string
	if json.Unmarshal(body.Content, &text) == nil {
		if message.Type == claudeAssistant && text != "" {
			d.lastText[scope] = text
			return []Event{{Kind: KindAgent, Scope: scope, Subject: text}}, nil
		}
		return nil, nil
	}
	var blocks []claudeBlock
	if err := json.Unmarshal(body.Content, &blocks); err != nil {
		return nil, err
	}
	var events []Event
	for _, block := range blocks {
		if message.Type == claudeUser {
			if block.Type == claudeToolResult {
				events = append(events, d.toolResult(scope, block))
			}
			continue
		}
		switch block.Type {
		case claudeText:
			// Different completed blocks may share one assistant message ID.
			// Accumulate them for final-result deduplication, but emit each block.
			if d.lastText[scope] != "" {
				d.lastText[scope] += "\n"
			}
			d.lastText[scope] += block.Text
			events = append(events, Event{Kind: KindAgent, Scope: scope, Subject: block.Text})
		case claudeThinking:
			events = append(events, Event{Kind: KindThinking, Scope: scope, Subject: block.Thinking})
		case claudeToolUse:
			call, err := decodeClaudeTool(scope, block)
			if err != nil {
				return nil, err
			}
			if _, exists := d.calls[call.key()]; !exists {
				d.calls[call.key()] = call
				events = append(events, call)
			}
		default:
			events = append(events, Event{Kind: KindNotice, Subject: "Unsupported " + AgentClaude + " content: " + string(block.Type)})
		}
	}
	return events, nil
}

func decodeClaudeTool(scope string, block claudeBlock) (Event, error) {
	if block.ID == "" {
		return Event{}, errors.New("missing tool-use ID")
	}
	var input struct {
		Command      json.RawMessage `json:"command"`
		FilePath     json.RawMessage `json:"file_path"`
		NotebookPath json.RawMessage `json:"notebook_path"`
		Path         json.RawMessage `json:"path"`
		Pattern      json.RawMessage `json:"pattern"`
		Query        json.RawMessage `json:"query"`
	}
	if len(block.Input) > 0 {
		if err := json.Unmarshal(block.Input, &input); err != nil {
			return Event{}, err
		}
	}
	e := Event{ID: block.ID, Scope: scope, Kind: KindTool, Phase: PhaseStarted, Subject: block.Name, Status: StatusRunning}
	switch block.Name {
	case claudeToolBash:
		e.Kind, e.Subject = KindCommand, jsonString(input.Command)
		if e.Subject == "" {
			e.Subject = block.Name
		}
	case claudeToolEdit, claudeToolWrite, claudeToolMultiEdit, claudeToolNotebookEdit:
		e.Kind, e.Subject = KindFile, jsonString(input.FilePath)
		if e.Subject == "" {
			e.Subject = jsonString(input.NotebookPath)
		}
		if e.Subject == "" {
			e.Subject = block.Name
		}
	default:
		// MCP tools own their schemas, so similarly named fields can contain
		// objects or numbers. Only use string values as optional descriptions.
		for _, raw := range []json.RawMessage{input.FilePath, input.Path, input.Pattern, input.Query} {
			target := jsonString(raw)
			if target != "" {
				e.Subject += " > " + target
				break
			}
		}
	}
	return e, nil
}

func (d *ClaudeDecoder) toolResult(scope string, block claudeBlock) Event {
	key := eventKey{scope: scope, id: block.ToolUseID}
	e, found := d.calls[key]
	delete(d.calls, key)
	if !found {
		e = Event{ID: block.ToolUseID, Scope: scope, Kind: KindTool, Subject: "Tool result " + block.ToolUseID}
	}
	e.Phase, e.Status, e.Output = PhaseCompleted, StatusCompleted, claudeResultText(block.Content)
	if block.IsError {
		e.Status = StatusFailed
	} else if e.Kind == KindFile {
		e.Detail = changeApplied
	}
	// Claude's text result is not a reliable numeric exit-code field. Leave
	// ExitCode unset rather than guessing from arbitrary command output.
	return e
}

func claudeResultText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return "Non-text tool output"
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == claudeText {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func decodeClaudeSystem(message claudeMessage) []Event {
	var events []Event
	for _, server := range message.MCPServerErrors {
		events = append(events, Event{Kind: KindNotice, Subject: "MCP server " + server.Name + ": " + server.Message})
	}
	switch message.Subtype {
	case claudeInit:
		return events
	case claudeAPIRetry:
		events = append(events, Event{Kind: KindNotice, Subject: fmt.Sprintf("API retry %d/%d in %.1fs", message.Attempt, message.MaxRetries, float64(message.RetryDelayMS)/1000)})
	default:
		text := errorMessage(message.Error)
		if text == "" {
			text = jsonString(message.Message)
		}
		if text == "" {
			text = message.Subtype
		}
		events = append(events, Event{Kind: KindNotice, Subject: text})
	}
	return events
}
