package transport

import "testing"

func TestParseDirectConnectURL(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		serverURL string
		authToken string
		wantErr   bool
	}{
		{
			name:      "cc url",
			raw:       "cc://127.0.0.1:54231/token-value",
			serverURL: "http://127.0.0.1:54231",
			authToken: "token-value",
		},
		{
			name:      "http url",
			raw:       "http://127.0.0.1:54231",
			serverURL: "http://127.0.0.1:54231",
		},
		{
			name:      "host without scheme",
			raw:       "127.0.0.1:54231",
			serverURL: "http://127.0.0.1:54231",
		},
		{
			name:    "unix socket unsupported",
			raw:     "cc+unix:///tmp/claude.sock",
			wantErr: true,
		},
		{
			name:    "empty",
			raw:     " ",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, err := ParseDirectConnectURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatal("ParseDirectConnectURL() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDirectConnectURL() error = %v", err)
			}
			if endpoint.ServerURL != tc.serverURL {
				t.Fatalf("ServerURL = %q, want %q", endpoint.ServerURL, tc.serverURL)
			}
			if endpoint.AuthToken != tc.authToken {
				t.Fatalf("AuthToken = %q, want %q", endpoint.AuthToken, tc.authToken)
			}
		})
	}
}
