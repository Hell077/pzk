package terminal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Hell077/portguard/internal/features/inspect"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type resultMsg struct {
	snapshot inspect.Snapshot
	err      error
}
type tickMsg time.Time

type model struct {
	ctx                   context.Context
	opts                  options
	snapshot              inspect.Snapshot
	err                   error
	width, height, cursor int
	loading, editing      bool
	query                 string
}

var (
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3E635"))
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8C939F"))
	bright   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E8ECF0")).Bold(true)
	warning  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3E635")).Background(lipgloss.Color("#202C19"))
)

func newModel(ctx context.Context, o options) model {
	return model{ctx: ctx, opts: o, query: o.query, width: 100, height: 30, loading: true}
}
func (m model) fetch() tea.Cmd {
	return func() tea.Msg { s, err := load(m.ctx, m.opts); return resultMsg{s, err} }
}
func (m model) tick() tea.Cmd {
	return tea.Tick(m.opts.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m model) Init() tea.Cmd { return tea.Batch(m.fetch(), m.tick()) }
func (m model) rows() inspect.Listeners {
	o := m.opts
	o.query = m.query
	return o.filter(m.snapshot.Listeners)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case resultMsg:
		// Keep the selected socket stable as rows appear or disappear.
		old := m.rows()
		key := ""
		if m.cursor < len(old) {
			key = identity(old[m.cursor])
		}
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.snapshot = msg.snapshot
		}
		rows := m.rows()
		m.cursor = min(m.cursor, max(0, len(rows)-1))
		for i, l := range rows {
			if identity(l) == key {
				m.cursor = i
				break
			}
		}
		return m, nil
	case tickMsg:
		if !m.loading {
			m.loading = true
			return m, tea.Batch(m.tick(), m.fetch())
		}
		return m, m.tick()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.editing {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyEnter:
				m.editing = false
			case tea.KeyBackspace, tea.KeyDelete:
				runes := []rune(m.query)
				if len(runes) > 0 {
					m.query = string(runes[:len(runes)-1])
				}
			case tea.KeyRunes:
				if len(m.query) < 256 {
					m.query += string(msg.Runes)
				}
			}
			m.cursor = 0
			return m, nil
		}
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "j", "down":
			m.cursor = min(m.cursor+1, max(0, len(m.rows())-1))
		case "k", "up":
			m.cursor = max(0, m.cursor-1)
		case "/":
			m.editing = true
		case "c":
			m.query = ""
			m.cursor = 0
		case "r":
			if !m.loading {
				m.loading = true
				return m, m.fetch()
			}
		}
	}
	return m, nil
}

func identity(l inspect.Listener) string {
	return fmt.Sprintf("%s/%s/%d", l.Protocol, l.Endpoint(), l.Process.PID)
}
func clip(s string, n int) string { return ansi.Truncate(Safe(s), max(0, n), "…") }
func cell(s string, n int) string {
	s = clip(s, n)
	return s + strings.Repeat(" ", max(0, n-lipgloss.Width(s)))
}

func (m model) View() string {
	w := max(10, m.width-4)
	if m.width < 48 || m.height < 16 {
		return "Portguard\nTerminal too small (48×16 minimum).\nUse --once or --json, or resize.\nq / ctrl+c to quit\n"
	}
	rows := m.rows()
	var b strings.Builder
	status := "● LIVE"
	if m.loading {
		status = "◌ SCANNING"
	}
	b.WriteString(bright.Render("◈ PORTGUARD") + "  " + muted.Render("know what owns your ports") + "\n")
	scope := ""
	if m.opts.low > 0 {
		scope = fmt.Sprintf(" · ports %d–%d", m.opts.low, m.opts.high)
	} else if m.opts.port > 0 {
		scope = fmt.Sprintf(" · port %d", m.opts.port)
	}
	b.WriteString(accent.Render(status) + muted.Render(fmt.Sprintf("   %d visible%s · %s · read-only", len(rows), scope, m.snapshot.Duration.Round(time.Millisecond))) + "\n\n")
	filter := "/ search owner, port, service, container"
	if m.query != "" || m.editing {
		filter = "/ " + m.query
	}
	if m.editing {
		filter += "▏"
	}
	b.WriteString(accent.Render(clip(filter, w)) + "\n\n")
	compact := w < 90
	tableRow := func(l inspect.Listener) string {
		name := l.Process.Name
		if name == "" {
			name = "unknown"
		}
		if compact {
			return cell(fmt.Sprint(l.Port), 6) + cell(strings.ToUpper(l.Protocol), 5) + cell(name, max(8, w-22)) + cell(fmt.Sprint(l.Process.PID), 9)
		}
		context := l.Service
		if context == "" {
			context = l.Process.ParentName
		}
		if len(l.Containers) > 0 {
			context = l.Containers[0].Engine + ":" + l.Containers[0].Name
		}
		return cell(l.Endpoint(), 26) + cell(strings.ToUpper(l.Protocol), 6) + cell(fmt.Sprint(l.Process.PID), 9) + cell(name, 20) + cell(context, w-61)
	}
	header := "PORT  PROTO OWNER / PID"
	if !compact {
		header = cell("LOCAL ENDPOINT", 26) + cell("PROTO", 6) + cell("PID", 9) + cell("PROCESS", 20) + "CONTEXT"
	}
	b.WriteString(muted.Render(clip(header, w)) + "\n")
	count := max(1, min(12, m.height-20))
	start := max(0, m.cursor-count+1)
	if len(rows) == 0 {
		if m.loading {
			b.WriteString(muted.Render("Discovering local sockets…"))
		} else {
			b.WriteString(muted.Render("No matching listeners visible to this user."))
		}
		b.WriteByte('\n')
	}
	for i := start; i < min(len(rows), start+count); i++ {
		row := tableRow(rows[i])
		if i == m.cursor {
			row = selected.Render(row)
		}
		b.WriteString(row + "\n")
	}
	if len(rows) > count {
		b.WriteString(muted.Render(fmt.Sprintf("%d–%d of %d · ↑/↓ to scroll", start+1, min(start+count, len(rows)), len(rows))) + "\n")
	}
	b.WriteByte('\n')
	if len(rows) > 0 {
		l := rows[min(m.cursor, len(rows)-1)]
		b.WriteString(bright.Render(clip(l.Reason(), w)) + "\n")
		b.WriteString(muted.Render(clip(fmt.Sprintf("Parent  %s (%d)   User  %s", l.Process.ParentName, l.Process.ParentPID, l.Process.User), w)) + "\n")
		executable := l.Process.Executable
		if executable == "" {
			executable = "unavailable"
		}
		b.WriteString(muted.Render(clip("Path    "+executable, w)) + "\n")
		service := l.Service
		if service == "" {
			service = "not resolved"
		}
		b.WriteString(muted.Render(clip("Service "+service+"   Exposure  "+l.Exposure(), w)) + "\n")
		for _, c := range l.Containers {
			b.WriteString(accent.Render(clip("Container "+c.Name+" · "+c.Image+" · "+c.Service, w)) + "\n")
			break
		}
	}
	b.WriteByte('\n')
	if m.err != nil {
		b.WriteString(warning.Render(clip("Scan failed: "+m.err.Error(), w)) + "\n")
	}
	if len(m.snapshot.Warnings) > 0 {
		b.WriteString(warning.Render(clip("! "+strings.Join(m.snapshot.Warnings, " · "), w)) + "\n")
	}
	b.WriteString(warning.Render(clip("Inspect the owner before exposing a port or restarting its service.", w)) + "\n")
	b.WriteString(muted.Render(clip("↑↓ / jk navigate   / filter   c clear   r refresh   q quit", w)))
	// Cap every line and viewport height, including unusually small resized terminals.
	lines := strings.Split(b.String(), "\n")
	if len(lines) > m.height-2 {
		lines = lines[:m.height-2]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], w, "")
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}
