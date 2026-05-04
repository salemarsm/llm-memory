package config

import "testing"

func TestValidateRequiresAuthForNonLoopback(t *testing.T) {
	cfg := Default()
	cfg.Server.Addr = "0.0.0.0:8787"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-loopback bind without auth to fail validation")
	}
	cfg.Server.AuthToken = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected auth token to allow non-loopback bind: %v", err)
	}
}

func TestLoopbackNoAuthAllowed(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8787", "localhost:8787", "[::1]:8787"} {
		cfg := Default()
		cfg.Server.Addr = addr
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s should allow no-auth loopback mode: %v", addr, err)
		}
	}
}
