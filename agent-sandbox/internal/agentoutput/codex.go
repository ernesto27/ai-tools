package agentoutput

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/ernesto27/ai-tools/agent-sandbox/agentoutput"
)

type CodexDecoder struct{}

func NewCodexDecoder() *CodexDecoder { return &CodexDecoder{} }
func (*CodexDecoder) Name() string   { return AgentCodex }

type codexMessage struct {
	Type    codexEventType          `json:"type"`
	Item    codexItem               `json:"item"`
	Message string                  `json:"message"`
	Error   json.RawMessage         `json:"error"`
	Usage   *agentoutput.TokenUsage `json:"usage"`
}

type codexItem struct {
	ID               string          `json:"id"`
	Type             codexItemType   `json:"type"`
	Command          string          `json:"command"`
	Status           Status          `json:"status"`
	ExitCode         *int            `json:"exit_code"`
	AggregatedOutput string          `json:"aggregated_output"`
	Text             string          `json:"text"`
	Message          string          `json:"message"`
	Server           string          `json:"server"`
	Tool             string          `json:"tool"`
	Query            string          `json:"query"`
	Error            json.RawMessage `json:"error"`
	Changes          []struct {
		Path string `json:"path"`
	} `json:"changes"`
	Items []struct {
		Text      string `json:"text"`
		Completed bool   `json:"completed"`
	} `json:"items"`
}

func (*CodexDecoder) Decode(line []byte) ([]Event, error) {
	var message codexMessage
	if err := json.Unmarshal(line, &message); err != nil {
		return nil, err
	}
	if message.Type == "" {
		return nil, errors.New("missing event type")
	}
	switch message.Type {
	case codexThreadStarted, codexTurnStarted:
		return nil, nil
	case codexTurnCompleted:
		if message.Usage == nil {
			return nil, nil
		}
		// Cached tokens are already included in input tokens.
		message.Usage.TotalTokens = message.Usage.InputTokens + message.Usage.OutputTokens
		return []Event{{Kind: KindUsage, Usage: message.Usage}}, nil
	case codexTurnFailed, codexError:
		text := errorMessage(message.Error)
		if text == "" {
			text = message.Message
		}
		if text == "" {
			text = string(message.Type)
		}
		return []Event{{Kind: KindFailure, Subject: text}}, nil
	case codexItemStarted, codexItemUpdated, codexItemCompleted:
		return decodeCodexItem(message.Type, message.Item), nil
	default:
		return []Event{{Kind: KindNotice, Subject: "Unsupported " + AgentCodex + " event: " + string(message.Type)}}, nil
	}
}

func decodeCodexItem(kind codexEventType, i codexItem) []Event {
	e := Event{ID: i.ID, Phase: PhaseCompleted, Status: i.Status}
	switch i.Type {
	case codexCommandExecution:
		e.Kind, e.Subject, e.ExitCode, e.Output = KindCommand, i.Command, i.ExitCode, i.AggregatedOutput
	case codexMCPToolCall:
		e.Kind, e.Subject, e.Output = KindTool, i.Server+" > "+i.Tool, errorMessage(i.Error)
		if e.Output != "" {
			e.Status = StatusFailed
		}
	default:
		if kind != codexItemCompleted {
			return nil
		}
	}
	if kind == codexItemStarted {
		e.Phase = PhaseStarted
		return []Event{e}
	}
	if kind != codexItemCompleted {
		return nil
	}
	switch i.Type {
	case codexCommandExecution, codexMCPToolCall:
		return []Event{e}
	case codexFileChange:
		e.Kind, e.Detail = KindFile, strings.TrimSpace("patch "+string(i.Status))
		if i.Status == StatusCompleted {
			e.Detail = patchApplied
		}
		if len(i.Changes) == 0 {
			return []Event{e}
		}
		changes := make([]Event, 0, len(i.Changes))
		for _, change := range i.Changes {
			e.Subject = change.Path
			changes = append(changes, e)
		}
		return changes
	case codexAgentMessage:
		e.Kind, e.Subject = KindAgent, i.Text
	case codexReasoning:
		e.Kind, e.Subject = KindThinking, i.Text
	case codexWebSearch:
		e.Kind, e.Subject = KindSearch, i.Query
	case codexTodoList:
		e.Kind = KindPlan
		var plan []string
		for _, task := range i.Items {
			mark := "[ ] "
			if task.Completed {
				mark = "[x] "
			}
			plan = append(plan, mark+task.Text)
		}
		e.Subject = strings.Join(plan, "\n")
	case codexItemError:
		e.Kind, e.Subject = KindFailure, i.Message
		if e.Subject == "" {
			e.Subject = i.Text
		}
	default:
		e.Kind, e.Subject = KindNotice, "Unsupported "+AgentCodex+" item: "+string(i.Type)
	}
	return []Event{e}
}
