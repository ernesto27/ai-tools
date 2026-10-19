package main

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/openrouter"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

func newModelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Browse model information from OpenRouter",
		Long:  "Browse models by creator, including descriptions, context limits, and pricing.",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			input, inOK := cmd.InOrStdin().(*os.File)
			output, outOK := cmd.OutOrStdout().(*os.File)
			if !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
				return sandbox.NewUsageError(fmt.Errorf("models requires an interactive terminal for stdin and stdout"))
			}
			_, err := tea.NewProgram(newModelBrowser(), tea.WithContext(cmd.Context()),
				tea.WithInput(input), tea.WithOutput(output)).Run()
			return err
		},
	}
}

type modelProvider string

func (p modelProvider) FilterValue() string { return string(p) }
func (p modelProvider) Title() string       { return string(p) }
func (p modelProvider) Description() string { return "" }

type previewModel struct {
	id, description, created, context, input, output string
}

var modelDescriptionLinks = regexp.MustCompile(`!?\[([^\]]+)\]\((?:[^()]|\([^()]*\))*\)`)

func modelDescription(raw string) string {
	// Keep Markdown link labels so descriptions remain readable in the terminal.
	text := modelDescriptionLinks.ReplaceAllString(ansi.Strip(raw), "$1")
	text = strings.NewReplacer("**", "", "__", "", "`", "").Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

type modelBrowser struct {
	providers       list.Model
	models          []previewModel
	catalog         viewport.Model
	selectedModel   int
	modelOffsets    []int
	providerFocused bool
	width, height   int

	client    *openrouter.OpenRouter
	loading   bool
	loadErr   error
	requestID uint64
}

func newModelBrowser() modelBrowser {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	providers := list.New([]list.Item{modelProvider("OpenAI"), modelProvider("Anthropic"), modelProvider("Google")}, delegate, 24, 20)
	providers.Title = "Providers"
	providers.SetShowStatusBar(false)
	providers.SetFilteringEnabled(false)
	providers.SetShowTitle(false)
	providers.SetShowHelp(false)
	providers.DisableQuitKeybindings()
	catalog := viewport.New()
	catalog.FillHeight = true

	m := modelBrowser{
		providers:       providers,
		catalog:         catalog,
		providerFocused: true,
		width:           100,
		height:          30,
		client:          openrouter.NewOpenRouter(),
		loading:         true,
		requestID:       1,
	}

	m.resize()
	return m
}

func (m *modelBrowser) selectProvider() tea.Cmd {
	provider := m.providers.SelectedItem().(modelProvider)

	m.requestID++
	m.loading = true
	m.loadErr = nil
	m.models = nil
	m.selectedModel = 0
	m.renderCatalog()
	m.catalog.GotoTop()

	return fetchModels(
		m.client,
		strings.ToLower(string(provider)),
		m.requestID,
	)
}

func (m *modelBrowser) resize() {
	sidebar, catalog, bodyHeight := m.dimensions()
	m.providers.SetSize(max(1, sidebar-4), max(1, bodyHeight-4))
	m.catalog.SetWidth(max(1, catalog-4))
	m.catalog.SetHeight(max(1, bodyHeight-5))
	m.providers.SetDelegate(providerDelegate{colors: modelsColors(), focused: m.providerFocused})
	m.renderCatalog()
}

func (m modelBrowser) Init() tea.Cmd {
	provider := m.providers.SelectedItem().(modelProvider)

	return fetchModels(
		m.client,
		strings.ToLower(string(provider)),
		m.requestID,
	)
}

func (m modelBrowser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "shift+tab":
			m.providerFocused = !m.providerFocused
			m.resize()
			return m, nil
		case "left", "right":
			m.providerFocused = msg.String() == "left"
			m.resize()
			return m, nil
		case "enter":
			if m.providerFocused {
				m.providerFocused = false
				m.resize()
			}
			return m, nil
		}
		if !m.providerFocused {
			switch msg.String() {
			case "up", "k":
				m.selectModel(m.selectedModel - 1)
				return m, nil
			case "down", "j":
				m.selectModel(m.selectedModel + 1)
				return m, nil
			case "home", "g":
				m.selectModel(0)
				return m, nil
			case "end", "G":
				m.selectModel(len(m.models) - 1)
				return m, nil
			}
		}
	case modelsLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		m.loading = false
		m.loadErr = msg.err

		if msg.err != nil {
			return m, nil
		}

		items := make([]previewModel, 0, len(msg.models))
		for _, model := range msg.models {
			items = append(items, previewModel{
				id:          model.ID,
				description: modelDescription(model.Description),
				created:     formatCreated(model.Created),
				context:     formatContext(model.ContextLength),
				input:       strings.TrimSuffix(model.InputPrice, " / 1M tokens"),
				output:      strings.TrimSuffix(model.OutputPrice, " / 1M tokens"),
			})
		}

		m.models = items
		m.selectedModel = 0
		m.renderCatalog()
		m.catalog.GotoTop()
		return m, nil
	}
	var cmd tea.Cmd
	if m.providerFocused {
		previous := m.providers.Index()
		m.providers, cmd = m.providers.Update(msg)
		if previous != m.providers.Index() {
			return m, tea.Batch(cmd, m.selectProvider())
		}
	} else {
		m.catalog, cmd = m.catalog.Update(msg)
	}
	return m, cmd
}

func (m modelBrowser) View() tea.View {
	if m.width < 64 || m.height < 18 {
		v := tea.NewView("Resize to at least 64 × 18.\nq: quit")
		v.AltScreen = true
		return v
	}
	p := modelsColors()
	sidebar, catalog, bodyHeight := m.dimensions()
	panel := func(title, subtitle, content string, width int, focused bool) string {
		border := p.border
		marker := "○ "
		if focused {
			border = p.accent
			marker = "● "
		}
		heading := lipgloss.NewStyle().Foreground(p.accent).Render(marker) + lipgloss.NewStyle().Bold(true).Foreground(p.text).Render(title)
		sub := lipgloss.NewStyle().Foreground(p.muted).Render(subtitle)
		return lipgloss.NewStyle().Width(width).Height(bodyHeight).
			Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).
			Render(heading + "\n" + sub + "\n" + content)
	}
	provider := string(m.providers.SelectedItem().(modelProvider))
	left := panel("Providers", "Model creators", m.providers.View(), sidebar, m.providerFocused)
	columns := lipgloss.NewStyle().Foreground(p.muted).Render("  " + modelColumns("", "CONTEXT", "INPUT", "OUTPUT", catalog-6))
	subtitle := fmt.Sprintf("%02d models · USD / 1M tokens", len(m.models))
	content := columns + "\n" + m.catalog.View()
	switch {
	case m.loading:
		subtitle = "Loading models…"
		content = "Fetching models from OpenRouter…"
	case m.loadErr != nil:
		subtitle = "Request failed"
		content = ansi.Truncate(m.loadErr.Error(), catalog-4, "…")
	case len(m.models) == 0:
		content = "No models found for this provider."
	}
	center := panel(provider, subtitle, content, catalog, !m.providerFocused)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", center)
	help := " ←/→ panels   ↑/↓ select   PgUp/PgDn scroll   q quit"
	footer := lipgloss.NewStyle().Foreground(p.muted).Render(ansi.Truncate(help, m.width, "…"))
	v := tea.NewView(body + "\n" + footer)
	v.BackgroundColor = p.background
	v.ForegroundColor = p.text
	v.AltScreen = true
	return v
}

func (m modelBrowser) dimensions() (sidebar, catalog, bodyHeight int) {
	sidebar = 18
	catalog = m.width - sidebar - 1
	return sidebar, catalog, m.height - 1
}

func formatContext(tokens int) string {
	value, suffix := float64(tokens), ""
	switch {
	case tokens >= 1_000_000:
		value, suffix = value/1_000_000, "M"
	case tokens >= 1_000:
		value, suffix = value/1_000, "K"
	default:
		return fmt.Sprintf("%d", tokens)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".") + suffix
}

func formatCreated(timestamp int64) string {
	if timestamp <= 0 {
		return "—"
	}
	return time.Unix(timestamp, 0).UTC().Format(time.DateOnly)
}

func modelColumns(name, context, input, output string, width int) string {
	nameWidth := modelTextWidth(width)
	name = ansi.Truncate(name, nameWidth, "…")
	name += strings.Repeat(" ", max(0, nameWidth-lipgloss.Width(name)))
	return fmt.Sprintf("%s %7s %7s %7s", name, context, input, output)
}

func modelTextWidth(width int) int {
	// Reserve the same columns for context and prices on every row of an entry.
	return max(1, width-24)
}

type modelsLoadedMsg struct {
	requestID uint64
	models    []openrouter.Model
	err       error
}

func fetchModels(
	client *openrouter.OpenRouter,
	provider string,
	requestID uint64,
) tea.Cmd {
	return func() tea.Msg {
		models, err := client.GetModelsByProvider(provider)

		return modelsLoadedMsg{
			requestID: requestID,
			models:    models,
			err:       err,
		}
	}
}

func (m *modelBrowser) renderCatalog() {
	p := modelsColors()
	width := max(1, m.catalog.Width()-2)
	var lines []string
	m.modelOffsets = make([]int, len(m.models))
	for i, model := range m.models {
		if i > 0 {
			lines = append(lines, "")
		}
		m.modelOffsets[i] = len(lines)
		marker := "  "
		heading := lipgloss.NewStyle().Foreground(p.text).Bold(true)
		if i == m.selectedModel {
			marker = "› "
			heading = heading.Foreground(p.accent)
			if !m.providerFocused {
				heading = heading.Background(p.selection)
			}
		}
		row := marker + modelColumns(model.id, model.context, model.input, model.output, width)
		lines = append(lines, heading.Render(row))
		// Descriptions wrap within the model column, leaving the numeric columns clear.
		for _, line := range strings.Split(ansi.Wrap(model.description, modelTextWidth(width), ""), "\n") {
			lines = append(lines, lipgloss.NewStyle().Foreground(p.muted).Render("  "+line))
		}
		for _, line := range strings.Split(ansi.Wrap("Added "+model.created, modelTextWidth(width), ""), "\n") {
			lines = append(lines, lipgloss.NewStyle().Foreground(p.muted).Faint(true).Render("  "+line))
		}
	}
	m.catalog.SetContent(strings.Join(lines, "\n"))
}

func (m *modelBrowser) selectModel(index int) {
	if len(m.models) == 0 {
		return
	}
	m.selectedModel = max(0, min(index, len(m.models)-1))
	m.renderCatalog()
	start := m.modelOffsets[m.selectedModel]
	end := m.catalog.TotalLineCount()
	if m.selectedModel+1 < len(m.modelOffsets) {
		end = m.modelOffsets[m.selectedModel+1] - 1
	}
	if start < m.catalog.YOffset() || end > m.catalog.YOffset()+m.catalog.Height() {
		m.catalog.SetYOffset(start)
	}
}

type providerDelegate struct {
	colors  modelsPalette
	focused bool
}

func (providerDelegate) Height() int                         { return 1 }
func (providerDelegate) Spacing() int                        { return 0 }
func (providerDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d providerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	provider := item.(modelProvider)
	marker := "  "
	style := lipgloss.NewStyle().Foreground(d.colors.text)
	if index == m.Index() {
		marker = "› "
		style = style.Foreground(d.colors.accent)
		if d.focused {
			style = style.Background(d.colors.selection)
		}
	}
	name := marker + string(provider)
	name += strings.Repeat(" ", max(0, m.Width()-lipgloss.Width(name)))
	fmt.Fprint(w, style.Bold(true).Render(name))
}

type modelsPalette struct {
	text, muted, accent, border, selection, background color.Color
}

func modelsColors() modelsPalette {
	return modelsPalette{
		text:       lipgloss.Color("#E5E5E5"),
		muted:      lipgloss.Color("#909090"),
		accent:     lipgloss.Color("#8AB4F8"),
		border:     lipgloss.Color("#303030"),
		selection:  lipgloss.Color("#1A1A1A"),
		background: lipgloss.Color("#000000"),
	}
}
