package main

import "testing"

// TestValidateServerPort pins the boot guard that stops the ":0" silent
// misconfiguration: PORT=0 must be refused, because net/http would bind a
// random free port and boot cleanly while every client waiting on the
// expected port times out ("server tidak bisa dijangkau" on the client).
func TestValidateServerPort(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "canonical dev port", raw: "8080", wantErr: false},
		{name: "explicit other port", raw: "3000", wantErr: false},
		{name: "highest valid port", raw: "65535", wantErr: false},
		{name: "empty is invalid", raw: "", wantErr: true},
		{name: "zero means random port", raw: "0", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "above range", raw: "65536", wantErr: true},
		{name: "not a number", raw: "http", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerPort(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("validateServerPort(%q) = nil, want error", tc.raw)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateServerPort(%q) = %v, want nil", tc.raw, err)
			}
		})
	}
}
