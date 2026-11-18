//go:build linux

package transport

import "testing"

func TestParseProcessStat(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		state   byte
		session int
	}{
		{"running", "42 (sleep) S 1 42 40 0 -1", 'S', 40},
		{"zombie", "43 (sleep) Z 1 43 40 0 -1", 'Z', 40},
		{"comm with spaces and parens", "44 (a b) (c)) R 1 44 41 0 -1", 'R', 41},
	}
	for _, tc := range cases {
		state, session, err := parseProcessStat([]byte(tc.data))
		if err != nil || state != tc.state || session != tc.session {
			t.Fatalf("%s: got %q %d %v, want %q %d", tc.name, state, session, err, tc.state, tc.session)
		}
	}
	for _, bad := range []string{"45 sleep S 1", "46 (sleep) S 1"} {
		if _, _, err := parseProcessStat([]byte(bad)); err == nil {
			t.Fatalf("parseProcessStat(%q) error = nil", bad)
		}
	}
}
