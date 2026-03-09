package ui

import (
	"fmt"
	"strings"

	"atlas.cat/internal/viewer"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#D4AF37")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4AF37")).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	matchCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#D4AF37")).
			Padding(0, 1).
			Bold(true)
)

type Model struct {
	processor *viewer.Processor
	viewport  viewport.Model
	ready     bool

	// Search
	searching   bool
	searchInput textinput.Model
	searchQuery string
	matches     []int
	matchIndex  int
}

func NewModel(p *viewer.Processor) Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Prompt = " / "
	ti.Focus()

	return Model{
		processor:   p,
		searchInput: ti,
		matchIndex:  -1,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	if m.searching {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.searchQuery = m.searchInput.Value()
				m.searching = false
				m.performSearch()
				m.updateContent()
				return m, nil
			case "esc":
				m.searching = false
				m.searchInput.Reset()
				return m, nil
			}
		}
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "l":
			m.processor.ShowLineNumbers = !m.processor.ShowLineNumbers
			m.updateContent()
		case "H":
			m.processor.HexMode = !m.processor.HexMode
			m.updateContent()
		case "w":
			m.processor.WrapLines = !m.processor.WrapLines
			m.updateContent()
		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "n":
			m.findNext()
			m.updateContent()
			return m, nil
		case "N", "p":
			m.findPrev()
			m.updateContent()
			return m, nil
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		m.processor.ViewportWidth = msg.Width
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.updateContent()
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.updateContent()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateContent() {
	m.viewport.SetContent(m.processor.HighlightAll(m.searchQuery, m.matchIndex))
}

func (m *Model) performSearch() {
	if m.searchQuery == "" {
		m.matches = nil
		m.matchIndex = -1
		return
	}

	m.matches = nil
	plain := m.processor.GetPlain()
	lowerPlain := strings.ToLower(plain)
	lowerQuery := strings.ToLower(m.searchQuery)

	start := 0
	for {
		idx := strings.Index(lowerPlain[start:], lowerQuery)
		if idx == -1 {
			break
		}
		m.matches = append(m.matches, start+idx)
		start += idx + len(lowerQuery)
	}

	if len(m.matches) > 0 {
		m.matchIndex = 0
		m.jumpToMatch()
	} else {
		m.matchIndex = -1
	}
}

func (m *Model) findNext() {
	if len(m.matches) == 0 { return }
	m.matchIndex = (m.matchIndex + 1) % len(m.matches)
	m.jumpToMatch()
}

func (m *Model) findPrev() {
	if len(m.matches) == 0 { return }
	m.matchIndex = (m.matchIndex - 1 + len(m.matches)) % len(m.matches)
	m.jumpToMatch()
}

func (m *Model) jumpToMatch() {
	if m.matchIndex < 0 { return }
	offset := m.matches[m.matchIndex]
	plain := m.processor.GetPlain()
	lineNum := strings.Count(plain[:offset], "\n")
	m.viewport.SetYOffset(lineNum)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m Model) headerView() string {
	title := titleStyle.Render("ATLAS CAT - " + m.processor.Path)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, infoStyle.Render(line))
}

func (m Model) footerView() string {
	if m.searching {
		return m.searchInput.View()
	}

	percent := fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100)
	matchInfo := ""
	if len(m.matches) > 0 {
		matchInfo = matchCountStyle.Render(fmt.Sprintf("%d/%d", m.matchIndex+1, len(m.matches))) + " "
	}

	help := lipgloss.JoinHorizontal(lipgloss.Top,
		helpKeyStyle.Render(" q "), helpDescStyle.Render("quit "),
		helpKeyStyle.Render(" l "), helpDescStyle.Render("lines "),
		helpKeyStyle.Render(" H "), helpDescStyle.Render("hex "),
		helpKeyStyle.Render(" w "), helpDescStyle.Render("wrap "),
		helpKeyStyle.Render(" / "), helpDescStyle.Render("search "),
		helpKeyStyle.Render(" n/N "), helpDescStyle.Render("next/prev "),
	)

	gap := max(0, m.viewport.Width-lipgloss.Width(help)-lipgloss.Width(percent)-lipgloss.Width(matchInfo)-2)
	line := strings.Repeat(" ", gap)
	
	return lipgloss.JoinHorizontal(lipgloss.Center, help, line, matchInfo, infoStyle.Render(percent))
}

func max(a, b int) int {
	if a > b { return a }
	return b
}
