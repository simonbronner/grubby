package ui

import (
	"testing"
)

func TestSanitizeTmuxSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"auth-service", "auth-service"},
		{"api.gateway", "api-gateway"},
		{"core/payments", "core-payments"},
		{"weird@#name!123", "weird-name-123"},
		{"", "repo"},
		{"---", "repo"},
	}

	for _, tt := range tests {
		got := SanitizeTmuxSessionName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeTmuxSessionName(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
