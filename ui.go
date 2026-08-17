package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	stTitle   = lipgloss.NewStyle().Bold(true)
	stProcess = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	stDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	stError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	stWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
)

// promptModel is the confirmation popup. It shows which process owns the
// pane and waits for a single decision key.
type promptModel struct {
	paneID  string
	process string
	cmdline string
	width   int
	height  int

	closing bool  // "y" pressed, waiting for closePane to finish
	err     error // closePane failed; shown until any key
	done    bool  // program should exit
	// decision records what the user chose; tests assert on it.
	decision decision

	closeFn func() error
}

type decision int

const (
	decisionPending decision = iota
	decisionClose
	decisionKeep
)

// closedMsg reports the result of the pane close command.
type closedMsg struct{ err error }

func newPromptModel(paneID, process, cmdline string, closeFn func() error) promptModel {
	return promptModel{paneID: paneID, process: process, cmdline: cmdline, closeFn: closeFn}
}

func (m promptModel) Init() tea.Cmd { return nil }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case closedMsg:
		if msg.err != nil {
			m.closing = false
			m.err = msg.err
			return m, nil
		}
		m.done = true
		return m, tea.Quit
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m promptModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.closing {
		// Ignore input while the close request is in flight.
		return m, nil
	}
	if m.err != nil {
		// Any key dismisses the error and keeps the pane.
		m.decision = decisionKeep
		m.done = true
		return m, tea.Quit
	}
	switch msg.String() {
	case "y", "Y":
		m.decision = decisionClose
		m.closing = true
		fn := m.closeFn
		return m, func() tea.Msg { return closedMsg{err: fn()} }
	default:
		// n, N, esc, enter, q, ctrl+c and everything else keep the pane.
		m.decision = decisionKeep
		m.done = true
		return m, tea.Quit
	}
}

func (m promptModel) View() string {
	var b strings.Builder
	inner := m.width - 4 // one cell padding each side plus a little breathing room
	if inner < 20 {
		inner = 60
	}
	b.WriteString("\n")
	b.WriteString("  " + stTitle.Render("Close pane?") + "\n\n")

	switch {
	case m.err != nil:
		b.WriteString("  " + stError.Render("Could not close the pane") + "\n")
		b.WriteString("  " + stDim.Render(truncate(m.err.Error(), inner-2)) + "\n\n")
		b.WriteString("  " + stDim.Render("press any key to dismiss") + "\n")
	case m.closing:
		b.WriteString("  " + stDim.Render("closing…") + "\n")
	default:
		if m.process == unknownProcess {
			b.WriteString(fmt.Sprintf("  Pane %s is running %s\n",
				stTitle.Render(m.paneID), stWarn.Render("an unknown process")))
			b.WriteString("  " + stDim.Render("herdr could not inspect the pane; it may still be busy") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  Pane %s is running %s\n",
				stTitle.Render(m.paneID), stProcess.Render(m.process)))
			if m.cmdline != "" && m.cmdline != m.process {
				b.WriteString("  " + stDim.Render(truncate(m.cmdline, inner-2)) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString("  " + stKey.Render("y") + " close    " +
			stKey.Render("n") + stDim.Render(" / ") + stKey.Render("esc") + " keep\n")
	}
	return b.String()
}

// truncate shortens s to at most n cells, appending an ellipsis.
func truncate(s string, n int) string {
	if n <= 1 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// runPrompt starts the popup UI. It returns the user's decision so main can
// exit non-zero when the close failed.
func runPrompt(paneID, process, cmdline string, closeFn func() error) (decision, error) {
	m := newPromptModel(paneID, process, cmdline, closeFn)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return decisionPending, err
	}
	fm, ok := final.(promptModel)
	if !ok {
		return decisionPending, fmt.Errorf("unexpected final model %T", final)
	}
	if fm.err != nil {
		return fm.decision, fm.err
	}
	return fm.decision, nil
}
