package services

import "testing"

func TestFromCgroup(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"0::/system.slice/postgresql.service", "postgresql.service"}, {"0::/user.slice/user-1000.slice/user@1000.service/app.slice/dev.service", "dev.service"}, {"0::/docker/aabbcc", ""}} {
		if got := FromCgroup(tc.in); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
