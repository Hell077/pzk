package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"kill", "--help"}, {"inspect", "--help"}, {"--version"}} {
		var out, errOut bytes.Buffer
		if err := run(args, "v0.1.0", strings.NewReader(""), &out, &errOut); err != nil {
			t.Fatal(err)
		}
		if out.Len()+errOut.Len() == 0 {
			t.Fatal("missing output")
		}
	}
}
func TestInvalidArgumentsNeverDiscoverOrSignal(t *testing.T) {
	for _, args := range [][]string{{"kill", "0", "--yes"}, {"kill", "65536", "--force"}, {"kill"}, {"list", "--range", "9000-3000"}, {"list", "--range", "abc"}, {"inspect", "-1"}, {"wat"}, {"watch", "8080", "--interval", "1ms"}} {
		if err := run(args, "test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestSanitizeMetadata(t *testing.T) {
	if got := Safe("hello\x1b[2J\nworld\u009b"); strings.ContainsAny(got, "\x1b\n\u009b") {
		t.Fatal("terminal control code escaped sanitization")
	}
}
