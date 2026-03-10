package cmd

import "testing"

func TestValidateMCPPort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{name: "default", port: defaultMCPPort},
		{name: "lower bound", port: 1},
		{name: "upper bound", port: 65535},
		{name: "zero", port: 0, wantErr: true},
		{name: "negative", port: -1, wantErr: true},
		{name: "too large", port: 65536, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMCPPort(tc.port)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for port %d", tc.port)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for port %d: %v", tc.port, err)
			}
		})
	}
}
