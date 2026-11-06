package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DAYA_ADDR", "")
	t.Setenv("DAYA_READ_HEADER_TIMEOUT", "")
	t.Setenv("DAYA_SHUTDOWN_TIMEOUT", "")

	got := Load()
	if got.Addr != ":8100" {
		t.Fatalf("Addr = %q, want %q", got.Addr, ":8100")
	}
	if got.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 5s", got.ReadHeaderTimeout)
	}
	if got.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", got.ShutdownTimeout)
	}
}
