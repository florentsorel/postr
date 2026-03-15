package config_test

import (
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/config"
)

func TestLoad_AuthValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "auth enabled without user",
			env:     map[string]string{"AUTH_ENABLED": "true", "AUTH_PASS": "secret"},
			wantErr: "AUTH_USER",
		},
		{
			name:    "auth enabled without pass",
			env:     map[string]string{"AUTH_ENABLED": "true", "AUTH_USER": "admin"},
			wantErr: "AUTH_PASS",
		},
		{
			name: "auth enabled with both set",
			env:  map[string]string{"AUTH_ENABLED": "true", "AUTH_USER": "admin", "AUTH_PASS": "secret"},
		},
		{
			name: "auth disabled, credentials not required",
			env:  map[string]string{"AUTH_ENABLED": "false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := config.Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not mention %q", err.Error(), tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
