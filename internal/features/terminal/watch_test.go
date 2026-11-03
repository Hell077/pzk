package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Hell077/pzk/internal/features/inspect"
)

var errSnapshotCaptured = errors.New("snapshot captured")

// Stop streaming after the first complete JSON write, without a timer or OS signal.
type snapshotWriter struct{ bytes.Buffer }

func (w *snapshotWriter) Write(p []byte) (int, error) {
	w.Buffer.Write(p)
	return 0, errSnapshotCaptured
}

func TestWatchRangeCommandProducesFilteredSnapshot(t *testing.T) {
	var out snapshotWriter
	err := run([]string{"watch", "--range", "7999-8080", "--json", "--no-containers"}, "test", strings.NewReader(""), &out, &bytes.Buffer{})
	if !errors.Is(err, errSnapshotCaptured) {
		t.Fatalf("watch did not accept range: %v", err)
	}
	var snapshot inspect.Snapshot
	if err := json.Unmarshal(out.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	for _, l := range snapshot.Listeners {
		if l.Port < 7999 || l.Port > 8080 {
			t.Fatalf("out-of-range listener: %d", l.Port)
		}
	}
}

func TestWatchRangeIncludesBoundariesInTextAndTUI(t *testing.T) {
	ls := inspect.Listeners{{Port: 7998, Protocol: "tcp"}, {Port: 7999, Protocol: "tcp"}, {Port: 8080, Protocol: "udp"}, {Port: 8081, Protocol: "tcp"}}
	o := options{low: 7999, high: 8080, protocol: "all"}
	got := o.filter(ls)
	if len(got) != 2 || got[0].Port != 7999 || got[1].Port != 8080 {
		t.Fatalf("range is not inclusive: %+v", got)
	}
	m := model{opts: o, snapshot: inspect.Snapshot{Listeners: ls}, width: 100, height: 30}
	if rows := m.rows(); len(rows) != 2 {
		t.Fatalf("TUI did not apply range: %+v", rows)
	}
	if !strings.Contains(m.View(), "ports 7999–8080") {
		t.Fatal("watch scope missing from UI")
	}
}

func TestWatchRejectsAmbiguousOrInvalidSelection(t *testing.T) {
	for _, args := range [][]string{{"watch"}, {"watch", "8080", "--range", "7999-8080"}, {"watch", "--range", "8080-7999"}, {"watch", "--range", "0-8080"}, {"watch", "--range", "7999-65536"}} {
		if err := run(args, "test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	var help bytes.Buffer
	if err := run([]string{"watch", "--help"}, "test", strings.NewReader(""), &bytes.Buffer{}, &help); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(help.String(), "--range START-END") {
		t.Fatal("range usage missing")
	}
}
