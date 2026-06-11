package config_test

import (
	"strings"
	"testing"

	"github.com/matharnica/vakt/internal/config"
)

func baseValidConfig() config.Config {
	return config.Config{
		DBUrl:     "postgres://vakt:pass@localhost/vakt",
		RedisUrl:  "redis://localhost:6379",
		SecretKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
	}
}

func TestValidate_AllZeroKey(t *testing.T) {
	cfg := baseValidConfig()
	cfg.SecretKey = strings.Repeat("0", 64)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for all-zero key, got nil")
	}
}

func TestValidate_AllSameByte_NonZero(t *testing.T) {
	cfg := baseValidConfig()
	cfg.SecretKey = strings.Repeat("ab", 32) // all bytes = 0xab
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for all-identical-byte key, got nil")
	}
}

func TestValidate_ValidKey(t *testing.T) {
	cfg := baseValidConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config to pass, got: %v", err)
	}
}

func TestValidate_TooShortKey(t *testing.T) {
	cfg := baseValidConfig()
	cfg.SecretKey = "deadbeef"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for short key, got nil")
	}
}

func TestValidate_MissingDBUrl(t *testing.T) {
	cfg := baseValidConfig()
	cfg.DBUrl = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing DB URL, got nil")
	}
}
