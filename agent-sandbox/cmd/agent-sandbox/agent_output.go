package main

import (
	"io"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/agentoutput"
)

func agentOutputFormatter(name string) func(io.Writer, string) io.WriteCloser {
	var newDecoder func() agentoutput.Decoder
	switch name {
	case agentoutput.AgentCodex:
		newDecoder = func() agentoutput.Decoder { return agentoutput.NewCodexDecoder() }
	case agentoutput.AgentClaude:
		newDecoder = func() agentoutput.Decoder { return agentoutput.NewClaudeDecoder() }
	default:
		return nil
	}
	return func(out io.Writer, branch string) io.WriteCloser {
		return agentoutput.New(out, branch, newDecoder())
	}
}
