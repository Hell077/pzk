package terminal

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/Hell077/portguard/internal/features/terminate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fakeStopper struct {
	port           uint32
	force          bool
	prepared, runs int
	err            error
}

func (f *fakeStopper) Prepare(_ context.Context, p uint32) (terminate.Selection, error) {
	f.port = p
	f.prepared++
	return terminate.Selection{Port: p, Owners: []inspect.Process{{PID: 42, Name: "node", StartedAt: 100}, {PID: 43, Name: "worker", StartedAt: 200}}}, f.err
}
func (f *fakeStopper) Run(_ context.Context, s terminate.Selection, force bool, out io.Writer) error {
	f.runs++
	f.port = s.Port
	f.force = force
	return f.err
}
func stopModel(f *fakeStopper) model {
	m := newModel(context.Background(), options{protocol: "all"})
	m.loading = false
	m.stopper = f
	m.snapshot.Listeners = inspect.Listeners{{Port: 7999, Protocol: "tcp", Process: inspect.Process{Name: "node"}}, {Port: 8000, Protocol: "tcp", Process: inspect.Process{Name: "python"}}}
	m.cursor = 1
	return m
}
func key(m model, key tea.KeyMsg) (model, tea.Cmd) {
	updated, cmd := m.Update(key)
	return updated.(model), cmd
}
func prepared(t *testing.T, m model, shortcut tea.KeyType) (model, tea.Cmd) {
	t.Helper()
	m, cmd := key(m, tea.KeyMsg{Type: shortcut})
	if cmd == nil {
		t.Fatal("missing preparation")
	}
	updated, _ := m.Update(cmd())
	return updated.(model), nil
}
func TestSelectedPortRequiresTypedConfirmation(t *testing.T) {
	for _, shortcut := range []tea.KeyType{tea.KeyCtrlK, tea.KeyCtrlX} {
		f := &fakeStopper{}
		m, _ := prepared(t, stopModel(f), shortcut)
		if f.port != 8000 || f.runs != 0 || m.stop.phase != stopReview {
			t.Fatal("wrong target or premature shutdown")
		}
		if len(m.stop.selection.Owners) != 2 {
			t.Fatal("review must retain every owner")
		}
		m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil || f.runs != 0 {
			t.Fatal("empty confirmation accepted")
		}
		m, _ = key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("yes")})
		m, cmd = key(m, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil || m.stop.phase != stopRunning {
			t.Fatal("confirmed action did not start")
		}
		updated, _ := m.Update(cmd())
		m = updated.(model)
		if f.runs != 1 || f.force != (shortcut == tea.KeyCtrlX) || m.stop.phase != stopDone {
			t.Fatal("wrong shutdown mode")
		}
	}
}
func TestCancelAndStalePreparationNeverChangeTarget(t *testing.T) {
	f := &fakeStopper{}
	m := stopModel(f)
	m, old := key(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyEsc})
	m.cursor = 0
	m, current := key(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	updated, _ := m.Update(old())
	m = updated.(model)
	if m.stop.phase != stopPreparing || m.stop.port != 7999 {
		t.Fatal("stale result replaced new selection")
	}
	updated, _ = m.Update(current())
	m = updated.(model)
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.stop != nil || f.runs != 0 {
		t.Fatal("cancel signaled a process")
	}
}
func TestStopShortcutDoesNotActWhileSearchingOrOnEmptyResults(t *testing.T) {
	f := &fakeStopper{}
	m := stopModel(f)
	m.editing = true
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	if cmd != nil || m.stop != nil {
		t.Fatal("shortcut acted inside search")
	}
	m.editing = false
	m.query = "no matching owner"
	m, cmd = key(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	if cmd != nil || m.stop != nil {
		t.Fatal("shortcut acted without a selection")
	}
}
func TestStopErrorsAndConfirmationFitSmallViewport(t *testing.T) {
	f := &fakeStopper{}
	m, _ := prepared(t, stopModel(f), tea.KeyCtrlK)
	m.width = 48
	m.height = 16
	m.stop.selection.Warnings = []string{"metadata warning"}
	m.stop.message = "Type exactly yes, then press Enter."
	view := m.View()
	if lipgloss.Height(view) > m.height || !strings.Contains(view, "Esc cancels") {
		t.Fatalf("confirmation controls clipped: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > m.width {
			t.Fatal("confirmation overflowed width")
		}
	}
	m.width = 40
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("yes")})
	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("accepted confirmation when review was not visible")
	}
	f.err = errors.New("permission denied")
	failed, _ := prepared(t, stopModel(f), tea.KeyCtrlK)
	if failed.stop.phase != stopDone || !strings.Contains(failed.View(), "permission denied") || f.runs != 0 {
		t.Fatal("preparation error hidden")
	}
}
