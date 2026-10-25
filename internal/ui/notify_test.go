package ui

import (
	"testing"
	"time"
)

func TestSendNativeNotification(t *testing.T) {
	// Verify that SendNativeNotification executes without crashing or throwing errors
	SendNativeNotification("Grubby Test", "Unit Test", "Verification of notification banner")
	time.Sleep(100 * time.Millisecond)
}
