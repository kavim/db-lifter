package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kevinmacielmedeiros/db-lift/internal/restore"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981"))

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

type statusMsg restore.Status
type tickMsg time.Time

type Model struct {
	cancel   context.CancelFunc
	statusCh chan restore.Status
	status   restore.Status
	spinner  spinner.Model
	progress progress.Model
	start    time.Time
	done     bool
	width    int
}

func NewModel(statusCh chan restore.Status, cancel context.CancelFunc) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	return Model{
		cancel:   cancel,
		statusCh: statusCh,
		spinner:  s,
		progress: p,
		start:    time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForStatus(m.statusCh),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 80 {
			m.progress.Width = 80
		}

	case statusMsg:
		m.status = restore.Status(msg)
		switch m.status.Phase {
		case restore.PhaseDone, restore.PhaseError:
			m.done = true
			return m, tea.Quit
		case restore.PhaseStream:
			return m, tea.Batch(
				waitForStatus(m.statusCh),
				tickProgress(),
			)
		}
		return m, waitForStatus(m.statusCh)

	case tickMsg:
		if m.done {
			return m, nil
		}
		if m.status.Progress != nil {
			cmd := m.progress.SetPercent(m.status.Progress.Percent())
			return m, tea.Batch(cmd, tickProgress())
		}
		return m, tickProgress()

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// FinalStatus returns the last restore status the TUI received.
func (m Model) FinalStatus() restore.Status {
	return m.status
}

// Elapsed returns the wall-clock duration from TUI start to now.
func (m Model) Elapsed() time.Duration {
	return time.Since(m.start).Truncate(time.Millisecond)
}

func (m Model) View() string {
	header := titleStyle.Render("⚡ db-lift")
	elapsed := dimStyle.Render(fmt.Sprintf("elapsed: %s", time.Since(m.start).Truncate(time.Millisecond)))

	var body string

	switch m.status.Phase {
	case restore.PhaseInit:
		body = m.spinner.View() + infoStyle.Render(" Checking container...")

	case restore.PhaseDrop:
		body = m.spinner.View() + infoStyle.Render(" Dropping & recreating database...")

	case restore.PhaseStream:
		if m.status.Progress != nil && m.status.Progress.Indeterminate() {
			transferred := formatBytes(m.status.Progress.BytesRead()) + " (size unknown)"
			body = fmt.Sprintf(
				"%s Streaming dump... %s\n\n  %s\n\n  %s",
				m.spinner.View(),
				infoStyle.Render("—"),
				m.progress.View(),
				dimStyle.Render(transferred),
			)
			break
		}
		pct := 0.0
		var transferred string
		if m.status.Progress != nil {
			pct = m.status.Progress.Percent()
			transferred = formatBytes(m.status.Progress.BytesRead()) + " / " + formatBytes(m.status.Progress.Total())
		}
		body = fmt.Sprintf(
			"%s Streaming dump... %s\n\n  %s\n\n  %s",
			m.spinner.View(),
			infoStyle.Render(fmt.Sprintf("%.1f%%", pct*100)),
			m.progress.View(),
			dimStyle.Render(transferred),
		)

	case restore.PhaseDone:
		body = successStyle.Render("✔ Restore completed successfully!")

	case restore.PhaseError:
		body = errorStyle.Render(fmt.Sprintf("✘ Error: %v", m.status.Err))
	}

	return fmt.Sprintf("\n%s  %s\n\n%s\n", header, elapsed, body)
}

func waitForStatus(ch chan restore.Status) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return statusMsg(restore.Status{Phase: restore.PhaseDone})
		}
		return statusMsg(s)
	}
}

func tickProgress() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
