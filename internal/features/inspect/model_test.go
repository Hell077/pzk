package inspect

import "testing"

func TestFilterIncludesContextAndKeepsProtocol(t *testing.T) {
	ls := Listeners{{Port: 8080, Protocol: "tcp", Process: Process{Name: "node", ParentName: "npm"}, Containers: []Container{{Name: "web", Service: "site/frontend"}}}, {Port: 8080, Protocol: "udp", Process: Process{Name: "other"}}}
	if got := ls.Filter(8080, "tcp", "FRONTEND"); len(got) != 1 || got[0].Process.Name != "node" {
		t.Fatalf("unexpected filter: %+v", got)
	}
	if got := ls.Filter(0, "all", "npm"); len(got) != 1 {
		t.Fatal("parent not searchable")
	}
	ports := ls.Project(func(l Listener) uint32 { return l.Port })
	if len(ports) != 2 || ports[0] != 8080 {
		t.Fatal(ports)
	}
}
