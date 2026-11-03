package containers

import "testing"

func TestParsePublishedPorts(t *testing.T) {
	data := []byte(`[{"Id":"abc","Name":"/database","Config":{"Image":"postgres:17","Labels":{"com.docker.compose.project":"app","com.docker.compose.service":"db"}},"NetworkSettings":{"Ports":{"5432/tcp":[{"HostIp":"127.0.0.1","HostPort":"5433"}],"53/udp":null}}}]`)
	ports, err := Parse("docker", data)
	if err != nil || len(ports) != 1 || ports[0].Name != "database" || ports[0].Service != "app/db" || ports[0].HostPort != 5433 {
		t.Fatalf("ports=%+v err=%v", ports, err)
	}
	if _, err := Parse("docker", []byte(`[{"NetworkSettings":{"Ports":{"80/tcp":[{"HostPort":"99999"}]}}}]`)); err == nil {
		t.Fatal("out of range port accepted")
	}
}

func TestLocalContextOnly(t *testing.T) {
	for _, s := range []string{"tcp://host:2375", "ssh://server", "", "https://localhost:2376"} {
		if LocalEndpoint(s) {
			t.Fatalf("remote or unknown endpoint accepted: %s", s)
		}
	}
	for _, s := range []string{"unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine"} {
		if !LocalEndpoint(s) {
			t.Fatal(s)
		}
	}
}
