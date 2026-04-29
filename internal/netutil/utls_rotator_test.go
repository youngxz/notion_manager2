package netutil

import (
	"testing"
)

func TestGetRandomChromeProfile(t *testing.T) {
	profile, fullVer, majorVer := GetRandomChromeProfile()
	if fullVer == "" || majorVer == "" {
		t.Fatalf("Expected version strings, got empty")
	}
	if profile.Client == "" {
		t.Fatalf("Expected non-empty profile, got empty Client")
	}
}
