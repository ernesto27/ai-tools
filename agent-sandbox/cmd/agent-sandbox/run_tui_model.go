package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"agent-sandbox/internal/docker"
	"agent-sandbox/internal/sandbox"
)

type liveTickMsg struct{}

type liveModel struct {
	output, details viewport.Model
	width, height   int
	detailsFocused  bool
	darkBackground  bool
	cancelling      bool
	opts            sandbox.Options
	facts           sandbox.Event
	lines           []string
	bridge          *liveBridge
	job             *liveJob
	cancel          context.CancelFunc
}

func newLiveModel(opts sandbox.Options, bridge *liveBridge, job *liveJob, cancel context.CancelFunc) liveModel {
	m := liveModel{output: viewport.New(), details: viewport.New(), opts: opts,
		bridge: bridge, job: job, cancel: cancel, darkBackground: true,
		facts: sandbox.Event{Phase: "Preparing"}, lines: []string{""}}
	m.output.SoftWrap, m.details.SoftWrap = true, true
	m.resize(100, 30)
	return m
}

func (m liveModel) Init() tea.Cmd {
	// Docker work starts only once Bubble Tea has initialized the terminal.
	// An initialization failure cancels the waiting worker without starting it.
	close(m.job.start)
	return tea.Batch(liveTick(), tea.RequestBackgroundColor)
}

func liveTick() tea.Cmd {
	return tea.Tick(75*time.Millisecond, func(time.Time) tea.Msg { return liveTickMsg{} })
}

func (m liveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.darkBackground = msg.IsDark()
		m.refreshContent()
		return m, nil
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case liveTickMsg:
		finished := false
		select {
		case <-m.job.done:
			finished = true
		default:
		}
		text, events := m.bridge.drain(finished)
		follow := m.output.AtBottom()
		if text != "" {
			lines := strings.Split(text, "\n")
			m.lines[len(m.lines)-1] += lines[0]
			m.lines = append(m.lines, lines[1:]...)
		}
		for _, event := range events {
			m.acceptFacts(event)
		}
		if m.job.ctx.Err() != nil && !finished && !m.cancelling {
			m.cancelling = true
		}
		if text != "" || len(events) > 0 || m.cancelling || finished {
			m.refreshContent()
			if follow {
				m.output.GotoBottom()
			}
		}
		if finished {
			// The worker has drained output and completed its deferred cleanup.
			// Only then may restoring the terminal end this view.
			return m, tea.Quit
		}
		return m, liveTick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if !m.cancelling {
				m.cancelling = true
				m.cancel()
				m.refreshContent()
			}
			return m, nil
		case "tab":
			m.detailsFocused = !m.detailsFocused
			return m, nil
		}
	}
	var cmd tea.Cmd
	if m.detailsFocused {
		m.details, cmd = m.details.Update(msg)
	} else {
		m.output, cmd = m.output.Update(msg)
	}
	return m, cmd
}

func (m *liveModel) acceptFacts(event sandbox.Event) {
	for _, field := range []struct {
		dst *string
		src string
	}{
		{&m.facts.Phase, event.Phase}, {&m.facts.Repository, event.Repository},
		{&m.facts.BaseBranch, event.BaseBranch}, {&m.facts.Worktree, event.Worktree},
		{&m.facts.Image, event.Image}, {&m.facts.Version, event.Version},
	} {
		if field.src != "" {
			*field.dst = livePlain(field.src)
		}
	}
}

func (m *liveModel) refreshContent() {
	palette := liveColors(m.darkBackground)
	m.output.SetContent(liveOutput(m.lines, m.output.Width(), palette))
	m.details.SetContent(m.detailContent(m.details.Width(), palette))
}

func (m liveModel) detailContent(width int, palette livePalette) string {
	heading := lipgloss.NewStyle().Bold(true).Foreground(palette.text)
	labelStyle := lipgloss.NewStyle().Foreground(palette.muted)
	valueStyle := lipgloss.NewStyle().Foreground(palette.text)
	ruleStyle := lipgloss.NewStyle().Foreground(palette.border)
	field := func(label, value string) string {
		if value == "" {
			return ""
		}
		const labelWidth = 12
		lines := strings.Split(ansi.Wrap(livePlain(value), max(1, width-labelWidth-2), "/"), "\n")
		for i := range lines {
			prefix := strings.Repeat(" ", labelWidth)
			if i == 0 {
				prefix = labelStyle.Render(fmt.Sprintf("%-12s", label+":"))
			}
			lines[i] = "  " + prefix + valueStyle.Render(lines[i])
		}
		return strings.Join(lines, "\n")
	}
	paragraph := func(text string) string {
		lines := strings.Split(ansi.Wrap(livePlain(text), max(1, width-2), "/"), "\n")
		for i := range lines {
			lines[i] = "  " + valueStyle.Render(lines[i])
		}
		return strings.Join(lines, "\n")
	}
	join := func(fields ...string) string {
		var present []string
		for _, f := range fields {
			if f != "" {
				present = append(present, f)
			}
		}
		return strings.Join(present, "\n")
	}
	enabled := func(value bool) string {
		if value {
			return "enabled"
		}
		return "disabled"
	}
	model := m.opts.Model
	if model == "" {
		model = "agent default"
	}
	phase := m.facts.Phase
	if m.cancelling {
		phase = "Cancelling · waiting for cleanup"
	}
	source := "query"
	if m.opts.FilePrompt != "" {
		source = m.opts.FilePrompt
	}
	sections := []struct{ title, content string }{
		{"Query", paragraph(m.opts.Prompt)},
		{"Agent", join(field("Name", m.opts.Agent.Name()), field("Version", m.facts.Version), field("Model", model))},
		{"Execution", join(field("Status", phase), field("Mode", m.job.mode), field("Prompt", source),
			field("Push", enabled(m.opts.Push)), field("PR", enabled(m.opts.PR)), field("Network", enabled(m.opts.HostNetwork)))},
		{"Workspace", join(field("Branch", m.opts.Branch), field("Base branch", m.facts.BaseBranch),
			field("Directory", "/workspace"), field("Repository", m.facts.Repository))},
		{"Image", join(field("Sandbox", m.facts.Image), field("Base", m.opts.BaseImage))},
	}
	if m.facts.Worktree != "" {
		sections = append(sections, struct{ title, content string }{"Worktree", paragraph(m.facts.Worktree)})
	}
	if m.opts.CommitMessage != "" && (m.opts.Push || m.opts.PR) {
		sections = append(sections, struct{ title, content string }{"Commit message", paragraph(m.opts.CommitMessage)})
	}
	if m.opts.Agent.SupportsImages() && len(m.opts.Images) > 0 {
		sections = append(sections, struct{ title, content string }{"Attachments", paragraph(strings.Join(m.opts.Images, "\n"))})
	}
	rules := strings.TrimSpace(strings.TrimPrefix(m.opts.FullPrompt(), m.opts.Prompt))
	sections = append(sections, struct{ title, content string }{"House rules", paragraph(rules)})
	var blocks []string
	for _, section := range sections {
		if section.content == "" {
			continue
		}
		rule := strings.Repeat("─", max(0, width-lipgloss.Width(section.title)-1))
		blocks = append(blocks, heading.Render(section.title)+" "+ruleStyle.Render(rule)+"\n"+section.content)
	}
	return strings.Join(blocks, "\n\n")
}

func (m *liveModel) resize(width, height int) {
	follow := m.output.AtBottom()
	m.width, m.height = width, height
	bodyHeight := max(8, height)
	if width >= 90 {
		m.details.SetWidth(34)
		m.output.SetWidth(width - 47)
		m.details.SetHeight(bodyHeight - 4)
		m.output.SetHeight(bodyHeight - 4)
	} else {
		m.details.SetWidth(max(1, width-6))
		m.output.SetWidth(max(1, width-6))
		detailsHeight := min(8, (bodyHeight-8)/2)
		m.details.SetHeight(detailsHeight)
		m.output.SetHeight(bodyHeight - 8 - detailsHeight)
	}
	m.refreshContent()
	if follow {
		m.output.GotoBottom()
	}
	// Only the most recent dimensions matter. A resize must never block the
	// view while Docker is busy building an image or stopping a container.
	select {
	case <-m.job.sizes:
	default:
	}
	select {
	case m.job.sizes <- docker.TerminalSize{Width: max(1, m.output.Width()), Height: max(1, m.output.Height())}:
	default:
	}
}

func (m liveModel) View() tea.View {
	if m.width < 40 || m.height < 20 {
		lines := strings.Split(fmt.Sprintf("Terminal too small (%dx%d).\nResize to at least 40x20.\nq cancel", m.width, m.height), "\n")
		lines = lines[:min(len(lines), max(0, m.height))]
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], max(0, m.width), "")
		}
		view := tea.NewView(strings.Join(lines, "\n"))
		view.AltScreen = true
		return view
	}
	palette := liveColors(m.darkBackground)
	output := livePanel("Agent output", m.output, !m.detailsFocused, palette)
	details := livePanel("Run details", m.details, m.detailsFocused, palette)
	body := lipgloss.JoinHorizontal(lipgloss.Top, output, " ", details)
	if m.width < 90 {
		body = lipgloss.JoinVertical(lipgloss.Left, details, output)
	}
	view := tea.NewView(body)
	view.AltScreen = true
	return view
}
