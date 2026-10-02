package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestSchemaVersionGate pins the boot guard's decision rule: a database at the
// chain head is fine, a database ahead of it is tolerated (it may carry
// migrations this binary does not know about), and a database behind it is a
// hard refusal.
func TestSchemaVersionGate(t *testing.T) {
	cases := []struct {
		name     string
		current  int
		expected int
		wantErr  bool
	}{
		{name: "at chain head", current: 114, expected: 114},
		{name: "ahead of chain head", current: 115, expected: 114},
		{name: "behind chain head", current: 113, expected: 114, wantErr: true},
		{name: "never migrated", current: 0, expected: 114, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := schemaVersionGate(tc.current, tc.expected)
			if tc.wantErr && err == nil {
				t.Fatalf("schemaVersionGate(%d, %d) = nil, want error", tc.current, tc.expected)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("schemaVersionGate(%d, %d) = %v, want nil", tc.current, tc.expected, err)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), "behind") {
					t.Errorf("error %q should name the drift as behind the chain head", err)
				}
				if !strings.Contains(err.Error(), fmt.Sprintf("applied schema version %d", tc.current)) {
					t.Errorf("error %q should name the applied version %d", err, tc.current)
				}
				if !strings.Contains(err.Error(), fmt.Sprintf("chain head %d", tc.expected)) {
					t.Errorf("error %q should name the chain head %d", err, tc.expected)
				}
			}
		})
	}
}
