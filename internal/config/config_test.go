package config_test

import (
	"testing"
	"time"

	"github.com/jeboehm/invito/internal/config"
)

// setValidEnv populates all environment variables for a clean, hermetic test run.
func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("INVITO_SESSION_SECRET", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	t.Setenv("INVITO_BASE_URL", "https://example.com/")
	t.Setenv("INVITO_OIDC_ISSUER", "https://issuer.example.com")
	t.Setenv("INVITO_OIDC_CLIENT_ID", "client-id")
	t.Setenv("INVITO_OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("INVITO_SMTP_HOST", "smtp.example.com")
	t.Setenv("INVITO_SMTP_FROM", "noreply@example.com")
	// Clear optional vars so ambient dev env values don't bleed in.
	t.Setenv("INVITO_DB_PATH", "")
	t.Setenv("INVITO_LISTEN_ADDR", "")
	t.Setenv("INVITO_SMTP_PORT", "")
	t.Setenv("INVITO_SMTP_USER", "")
	t.Setenv("INVITO_SMTP_PASSWORD", "")
	t.Setenv("INVITO_SYNC_INTERVAL", "")
	t.Setenv("INVITO_BOOKING_TTL", "")
}

func TestLoad_Success(t *testing.T) {
	setValidEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://example.com" {
		t.Errorf("BaseURL trailing slash not trimmed: got %q", cfg.BaseURL)
	}
	if cfg.DBPath != "./invito.db" {
		t.Errorf("DBPath default: got %q, want ./invito.db", cfg.DBPath)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default: got %q, want :8080", cfg.ListenAddr)
	}
	if cfg.SyncInterval != 15*time.Minute {
		t.Errorf("SyncInterval default: got %v, want 15m", cfg.SyncInterval)
	}
	if cfg.BookingTTL != 24*time.Hour {
		t.Errorf("BookingTTL default: got %v, want 24h", cfg.BookingTTL)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort default: got %d, want 587", cfg.SMTPPort)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	required := []string{
		"INVITO_SESSION_SECRET",
		"INVITO_BASE_URL",
		"INVITO_OIDC_ISSUER",
		"INVITO_OIDC_CLIENT_ID",
		"INVITO_OIDC_CLIENT_SECRET",
		"INVITO_SMTP_HOST",
		"INVITO_SMTP_FROM",
	}
	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(key, "")
			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error when %s is missing", key)
			}
		})
	}
}

func TestLoad_InvalidSessionSecret_NotHex(t *testing.T) {
	setValidEnv(t)
	t.Setenv("INVITO_SESSION_SECRET", "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for non-hex session secret")
	}
}

func TestLoad_InvalidSessionSecret_WrongLength(t *testing.T) {
	setValidEnv(t)
	t.Setenv("INVITO_SESSION_SECRET", "0102030405060708")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for wrong-length session secret")
	}
}

func TestLoad_InvalidSMTPPort(t *testing.T) {
	setValidEnv(t)
	t.Setenv("INVITO_SMTP_PORT", "not-a-number")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid SMTP port")
	}
}

func TestLoad_InvalidSyncInterval(t *testing.T) {
	setValidEnv(t)
	t.Setenv("INVITO_SYNC_INTERVAL", "not-a-duration")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid sync interval")
	}
}

func TestLoad_SessionSecretDecoded(t *testing.T) {
	setValidEnv(t)
	t.Setenv("INVITO_SESSION_SECRET", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionSecret[0] != 0x01 || cfg.SessionSecret[31] != 0x20 {
		t.Errorf("session secret not decoded correctly: %x", cfg.SessionSecret)
	}
}
