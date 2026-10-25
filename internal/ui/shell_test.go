package ui

import (
	"testing"
)

func TestSanitizeTmuxSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"auth-service", "grubby-auth-service"},
		{"api.gateway", "grubby-api-gateway"},
		{"core/payments", "grubby-core-payments"},
		{"weird@#name!123", "grubby-weird-name-123"},
		{"", "grubby-repo"},
		{"---", "grubby-repo"},
	}

	for _, tt := range tests {
		got := SanitizeTmuxSessionName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeTmuxSessionName(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
