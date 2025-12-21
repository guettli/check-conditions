package checkconditions

import (
	"os"
	"testing"
)

func TestIsDebugMode(t *testing.T) {
	// Save the original value to restore after test
	originalValue := os.Getenv("CHECK_CONDITIONS_DEBUG")
	defer func() {
		if originalValue != "" {
			os.Setenv("CHECK_CONDITIONS_DEBUG", originalValue)
		} else {
			os.Unsetenv("CHECK_CONDITIONS_DEBUG")
		}
	}()

	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "env var not set",
			envValue: "",
			want:     false,
		},
		{
			name:     "env var set to 1",
			envValue: "1",
			want:     true,
		},
		{
			name:     "env var set to true",
			envValue: "true",
			want:     true,
		},
		{
			name:     "env var set to any value",
			envValue: "any_value",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("CHECK_CONDITIONS_DEBUG")
			} else {
				os.Setenv("CHECK_CONDITIONS_DEBUG", tt.envValue)
			}

			got := isDebugMode()
			if got != tt.want {
				t.Errorf("isDebugMode() = %v, want %v (env=%q)", got, tt.want, tt.envValue)
			}
		})
	}
}
