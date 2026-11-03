package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Hell077/pzk/internal/features/terminate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type portStopper interface {
	Prepare(context.Context, uint32) (terminate.Selection, error)
	Run(context.Context, terminate.Selection, bool, io.Writer) error
}
type nativeStopper struct{}

func (nativeStopper) Prepare(ctx context.Context, port uint32) (terminate.Selection, error) {
	return terminate.Prepare(ctx, port)
}
func (nativeStopper) Run(ctx context.Context, s terminate.Selection, force bool, out io.Writer) error {
	return terminate.RunSelection(ctx, s, force, out)
}

type stopPhase uint8

const (
	stopPreparing stopPhase = iota
	stopReview
	stopRunning
	stopDone
)

type stopAction struct {
	phase           stopPhase
	port            uint32
	force           bool
	selection       terminate.Selection
	answer, message string
	offset          int
}
type stopPreparedMsg struct {
	request   *stopAction
	selection terminate.Selection
	err       error
}
type stopFinishedMsg struct {
	output string
	err    error
}

func (m model) beginStop(force bool) (tea.Model, tea.Cmd) {
	rows := m.rows()
	if len(rows) == 0 || m.err != nil {
		return m, nil
	}
	port := rows[min(m.cursor, len(rows)-1)].Port
	m.stop = &stopAction{phase: stopPreparing, port: port, force: force}
	service := m.stopper
	if service == nil {
		service = nativeStopper{}
	}
	request := m.stop
	return m, func() tea.Msg { s, err := service.Prepare(m.ctx, port); return stopPreparedMsg{request, s, err} }
}
func (m model) preparedStop(msg stopPreparedMsg) (tea.Model, tea.Cmd) {
	if m.stop == nil || m.stop.phase != stopPreparing || m.stop != msg.request {
		return m, nil
	}
	if msg.err != nil {
		m.stop.phase = stopDone
		m.stop.message = "Cannot stop this port: " + msg.err.Error()
		return m, nil
	}
	if msg.selection.Port != m.stop.port || len(msg.selection.Owners) == 0 {
		m.stop.phase = stopDone
		m.stop.message = "Invalid owner selection; no process was signaled."
		return m, nil
	}
	m.stop.phase = stopReview
	m.stop.selection = msg.selection
	return m, nil
}
func (m model) finishedStop(msg stopFinishedMsg) (tea.Model, tea.Cmd) {
	if m.stop == nil || m.stop.phase != stopRunning {
		return m, nil
	}
	m.stop.phase = stopDone
	if msg.err != nil {
		m.stop.message = msg.err.Error()
	} else {
		m.stop.message = fmt.Sprintf("All reviewed owners of port %d exited. No visible owner remains.", m.stop.port)
	}
	return m, nil
}

func (m model) stopKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := m.stop
	if a.phase == stopRunning {
		return m, nil
	}
	if a.phase == stopDone {
		if msg.Type == tea.KeyUp {
			a.offset = max(0, a.offset-1)
		}
		if msg.Type == tea.KeyDown {
			a.offset++
		}
		if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEsc {
			m.stop = nil
			if !m.loading {
				m.loading = true
				return m, m.fetch()
			}
		}
		return m, nil
	}
	// Escape discards this preparation; request identity rejects any late result.
	if msg.Type == tea.KeyEsc {
		m.stop = nil
		return m, nil
	}
	if a.phase != stopReview || m.width < 48 || m.height < 16 {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		a.offset = max(0, a.offset-1)
	case tea.KeyDown:
		a.offset = min(max(0, len(a.selection.Owners)-1), a.offset+1)
	case tea.KeyBackspace, tea.KeyDelete:
		if len(a.answer) > 0 {
			r := []rune(a.answer)
			a.answer = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		if len(a.answer) < 16 {
			a.answer += string(msg.Runes)
		}
	case tea.KeyEnter:
		if a.answer != "yes" {
			a.message = "Type exactly yes, then press Enter. Escape cancels."
			return m, nil
		}
		a.phase = stopRunning
		a.message = ""
		service := m.stopper
		if service == nil {
			service = nativeStopper{}
		}
		selection, force := a.selection, a.force
		return m, func() tea.Msg {
			var out bytes.Buffer
			err := service.Run(m.ctx, selection, force, &out)
			return stopFinishedMsg{out.String(), err}
		}
	}
	return m, nil
}

func (m model) stopView() string {
	a := m.stop
	w := max(10, m.width-4)
	var b strings.Builder
	title := "GRACEFUL STOP"
	if a.force {
		title = "FORCE STOP · UNSAVED WORK MAY BE LOST"
	}
	b.WriteString(bright.Render("◈ PZK · PortZapKill") + "  " + warning.Render(clip(title, w-15)) + "\n\n")
	b.WriteString(bright.Render(fmt.Sprintf("Port %d · all TCP/UDP owners", a.port)) + "\n")
	switch a.phase {
	case stopPreparing:
		b.WriteString("\nResolving every owner and checking protections…\n\nEsc cancels. No signal has been sent.")
	case stopRunning:
		b.WriteString("\nRechecking identities and requesting shutdown…\n\nWaiting for process exit. Force never escalates automatically.")
	case stopDone:
		lines := strings.Split(lipgloss.NewStyle().Width(w).Render(Safe(a.message)), "\n")
		capacity := max(1, m.height-9)
		start := min(a.offset, max(0, len(lines)-capacity))
		b.WriteString("\n" + strings.Join(lines[start:min(len(lines), start+capacity)], "\n") + "\n\n↑↓ scroll · Enter / Esc returns to the live view.")
	case stopReview:
		b.WriteString(warning.Render("This affects every port each process owns.") + "\n")
		b.WriteString(muted.Render("A service manager may restart a stopped process.") + "\n\n")
		capacity := max(1, m.height-17)
		start := min(a.offset, max(0, len(a.selection.Owners)-capacity))
		end := min(len(a.selection.Owners), start+capacity)
		for _, p := range a.selection.Owners[start:end] {
			b.WriteString(clip(fmt.Sprintf("PID %-7d %s · parent %s (%d)", p.PID, p.Name, p.ParentName, p.ParentPID), w) + "\n")
		}
		b.WriteString(muted.Render(fmt.Sprintf("Owners %d–%d of %d · ↑↓ scroll", start+1, end, len(a.selection.Owners))) + "\n")
		if len(a.selection.Warnings) > 0 {
			b.WriteString(warning.Render(clip("! "+strings.Join(a.selection.Warnings, " · "), w)) + "\n")
		}
		b.WriteString("\n" + accent.Render("Type yes, then Enter: "+Safe(a.answer)+"▏") + "\n")
		if a.message != "" {
			b.WriteString(warning.Render(clip(a.message, w)) + "\n")
		}
		b.WriteString(muted.Render("Esc cancels. Owners are checked again before signaling."))
	}
	lines := strings.Split(b.String(), "\n")
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], w, "")
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}
