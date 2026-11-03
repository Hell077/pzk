package terminate

import (
	"bytes"
	"context"
	"errors"
	"github.com/Hell077/pzk/internal/features/inspect"
	"strings"
	"testing"
	"time"
)

type fakeTarget struct {
	signals int
	closed  bool
	alive   bool
	err     error
}

func (f *fakeTarget) Signal(force bool) error { f.signals++; return f.err }
func (f *fakeTarget) Alive() (bool, error)    { return f.alive, nil }
func (f *fakeTarget) Close() error            { f.closed = true; return nil }
func fixture(pid int32, start int64) inspect.Listener {
	return inspect.Listener{Port: 8080, Protocol: "tcp", Address: "127.0.0.1", Process: inspect.Process{PID: pid, Name: "test-worker", StartedAt: start}}
}
func snap(ls ...inspect.Listener) inspect.Snapshot { return inspect.Snapshot{Listeners: ls} }

func TestTerminationRechecksAllOwners(t *testing.T) {
	for _, tc := range []struct {
		name    string
		changed inspect.Snapshot
	}{
		{"reused PID", snap(fixture(42, 200))},
		{"new owner", snap(fixture(42, 100), fixture(43, 100))},
		{"owner disappeared", snap()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := &fakeTarget{}
			b := backend{snapshot: func(context.Context) (inspect.Snapshot, error) {
				calls++
				if calls == 1 {
					return snap(fixture(42, 100)), nil
				}
				return tc.changed, nil
			}, open: func(inspect.Process, bool) (target, error) { return h, nil }, timeout: time.Millisecond}
			err := b.run(context.Background(), 8080, false, true, strings.NewReader(""), &bytes.Buffer{})
			if err == nil || h.signals != 0 || !h.closed {
				t.Fatalf("err=%v signals=%d closed=%v", err, h.signals, h.closed)
			}
		})
	}
}

func TestAllHandlesPreparedBeforeAnySignal(t *testing.T) {
	first := &fakeTarget{}
	calls := 0
	b := backend{snapshot: func(context.Context) (inspect.Snapshot, error) { return snap(fixture(42, 100), fixture(43, 100)), nil }, open: func(inspect.Process, bool) (target, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("permission denied")
		}
		return first, nil
	}}
	err := b.run(context.Background(), 8080, true, true, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || first.signals != 0 || !first.closed {
		t.Fatalf("err=%v target=%+v", err, first)
	}
}
func TestConfirmationAndProtectedOwners(t *testing.T) {
	for _, answer := range []string{"", "y\n", "no\n"} {
		opened := false
		b := backend{snapshot: func(context.Context) (inspect.Snapshot, error) { return snap(fixture(42, 100)), nil }, open: func(inspect.Process, bool) (target, error) { opened = true; return &fakeTarget{}, nil }}
		if err := b.run(context.Background(), 8080, false, false, strings.NewReader(answer), &bytes.Buffer{}); err == nil || opened {
			t.Fatal("must cancel without opening process handles")
		}
	}
	for _, s := range []inspect.Snapshot{snap(fixture(0, 100)), snap(fixture(42, 0)), snap(fixture(1, 100)), snap(inspect.Listener{Port: 8080, Containers: []inspect.Container{{Name: "db"}}})} {
		if _, err := owners(s, 8080, map[int32]bool{1: true}); err == nil {
			t.Fatal("unsafe target accepted")
		}
	}
}
func TestDeduplicatesOwnersAndDetectsReoccupation(t *testing.T) {
	a := fixture(42, 100)
	b := a
	b.Protocol = "udp"
	h := &fakeTarget{}
	calls := 0
	engine := backend{snapshot: func(context.Context) (inspect.Snapshot, error) {
		calls++
		if calls <= 2 {
			return snap(a, b), nil
		}
		return snap(fixture(99, 300)), nil
	}, open: func(inspect.Process, bool) (target, error) { return h, nil }, timeout: time.Millisecond}
	err := engine.run(context.Background(), 8080, false, false, strings.NewReader("yes\n"), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "occupied again") || h.signals != 1 {
		t.Fatalf("err=%v signals=%d", err, h.signals)
	}
}
func TestTimeoutDoesNotEscalate(t *testing.T) {
	h := &fakeTarget{alive: true}
	b := backend{snapshot: func(context.Context) (inspect.Snapshot, error) { return snap(fixture(42, 100)), nil }, open: func(inspect.Process, bool) (target, error) { return h, nil }, timeout: time.Millisecond}
	err := b.run(context.Background(), 8080, false, true, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "timeout") || h.signals != 1 {
		t.Fatalf("err=%v signals=%d", err, h.signals)
	}
}

func TestProxyRejectedWithoutContainerMetadata(t *testing.T) {
	l := fixture(42, 100)
	l.Process.Name = "com.docker.backend"
	if _, err := owners(snap(l), 8080, nil); err == nil {
		t.Fatal("container backend accepted")
	}
}

func TestReviewedSelectionCannotSilentlyChangeTargets(t *testing.T) {
	opened := false
	b := backend{snapshot: func(context.Context) (inspect.Snapshot, error) { return snap(fixture(42, 200), fixture(43, 100)), nil }, reviewed: []inspect.Process{fixture(42, 100).Process}, open: func(inspect.Process, bool) (target, error) { opened = true; return &fakeTarget{}, nil }}
	err := b.run(context.Background(), 8080, false, true, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "reviewed") || opened {
		t.Fatalf("changed selection was accepted: err=%v opened=%v", err, opened)
	}
}
