package scan

import (
	"context"
	"github.com/Hell077/portguard/internal/features/inspect"
	"net"
	"os"
	"testing"
	"time"
)

func TestDiscoversRealListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := uint32(listener.Addr().(*net.TCPAddr).Port)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := Snapshot(ctx, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range s.Listeners {
		if l.Port == port && l.Protocol == "tcp" {
			if l.Process.PID != int32(os.Getpid()) || l.Process.StartedAt <= 0 {
				t.Fatalf("incorrect owner: %+v", l)
			}
			return
		}
	}
	t.Fatalf("did not discover local TCP listener on %d", port)
}
func TestContainerMappingsKeepProtocolAndAddress(t *testing.T) {
	ls := inspect.Listeners{{Port: 8080, Address: "127.0.0.1", Protocol: "tcp"}, {Port: 8080, Address: "127.0.0.1", Protocol: "udp"}}
	c := inspect.Container{Name: "web", HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: "80/tcp"}
	got := MergeContainers(ls, []inspect.Container{c})
	if len(got) != 2 || len(got[0].Containers) != 1 || len(got[1].Containers) != 0 {
		t.Fatalf("wrong merge: %+v", got)
	}
	c.HostIP = "0.0.0.0"
	got = MergeContainers(nil, []inspect.Container{c})
	if len(got) != 1 || got[0].Process.PID != 0 {
		t.Fatalf("virtual publication lost: %+v", got)
	}
}
