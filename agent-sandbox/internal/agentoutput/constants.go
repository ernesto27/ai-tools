package agentoutput

const (
	AgentCodex    = "codex"
	AgentClaude   = "claude"
	separator     = "---------------------------------------------\n"
	timeLayout    = "15:04:05"
	detailIndent  = "            "
	patchApplied  = "patch applied"
	changeApplied = "change applied"
)

type Kind string

const (
	KindUsage    Kind = "usage"
	KindCommand  Kind = "command"
	KindTool     Kind = "tool"
	KindFile     Kind = "file"
	KindAgent    Kind = "agent"
	KindThinking Kind = "thinking"
	KindSearch   Kind = "search"
	KindPlan     Kind = "plan"
	KindFailure  Kind = "failure"
	KindNotice   Kind = "notice"
)

type Phase uint8

const (
	PhaseStarted Phase = iota + 1
	PhaseCompleted
)

type Status string

const (
	StatusRunning   Status = "in_progress"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusDeclined  Status = "declined"
)

// Labels are shared with the TUI so color selection uses the same vocabulary.
const (
	LabelCommand   = "> COMMAND"
	LabelResult    = "+ RESULT"
	LabelTool      = "+ TOOL"
	LabelToolStart = "> TOOL"
	LabelFile      = "* FILE"
	LabelFailed    = "! FAILED"
	LabelOutput    = "! OUTPUT"
	LabelEvent     = "? EVENT"
	LabelAgent     = "  AGENT"
	LabelThinking  = "  THINKING"
	LabelSearch    = "+ SEARCH"
	LabelPlan      = "  PLAN"
)

type codexEventType string

const (
	codexThreadStarted codexEventType = "thread.started"
	codexTurnStarted   codexEventType = "turn.started"
	codexTurnCompleted codexEventType = "turn.completed"
	codexTurnFailed    codexEventType = "turn.failed"
	codexError         codexEventType = "error"
	codexItemStarted   codexEventType = "item.started"
	codexItemUpdated   codexEventType = "item.updated"
	codexItemCompleted codexEventType = "item.completed"
)

type codexItemType string

const (
	codexCommandExecution codexItemType = "command_execution"
	codexMCPToolCall      codexItemType = "mcp_tool_call"
	codexFileChange       codexItemType = "file_change"
	codexAgentMessage     codexItemType = "agent_message"
	codexReasoning        codexItemType = "reasoning"
	codexWebSearch        codexItemType = "web_search"
	codexTodoList         codexItemType = "todo_list"
	codexItemError        codexItemType = "error"
)

type claudeMessageType string

const (
	claudeAssistant    claudeMessageType = "assistant"
	claudeUser         claudeMessageType = "user"
	claudeResult       claudeMessageType = "result"
	claudeSystem       claudeMessageType = "system"
	claudeStreamEvent  claudeMessageType = "stream_event"
	claudeToolProgress claudeMessageType = "tool_progress"
)

type claudeBlockType string

const (
	claudeText       claudeBlockType = "text"
	claudeThinking   claudeBlockType = "thinking"
	claudeToolUse    claudeBlockType = "tool_use"
	claudeToolResult claudeBlockType = "tool_result"
)

const (
	claudeInit             = "init"
	claudeAPIRetry         = "api_retry"
	claudeErrorPrefix      = "error_"
	claudeToolBash         = "Bash"
	claudeToolEdit         = "Edit"
	claudeToolWrite        = "Write"
	claudeToolMultiEdit    = "MultiEdit"
	claudeToolNotebookEdit = "NotebookEdit"
)
