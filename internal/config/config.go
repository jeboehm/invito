package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL       string
	DBPath        string
	SessionSecret [32]byte
	ListenAddr    string

	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	SyncInterval time.Duration
	BookingTTL   time.Duration
}

func Load() (*Config, error) {
	var errs []string

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, key+" is required")
		}
		return v
	}

	optional := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	parseDuration := func(key, def string) time.Duration {
		s := optional(key, def)
		d, err := time.ParseDuration(s)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: invalid duration %q: %v", key, s, err))
			return 0
		}
		return d
	}

	secretHex := require("INVITO_SESSION_SECRET")
	baseURL := require("INVITO_BASE_URL")
	oidcIssuer := require("INVITO_OIDC_ISSUER")
	oidcClientID := require("INVITO_OIDC_CLIENT_ID")
	oidcClientSecret := require("INVITO_OIDC_CLIENT_SECRET")
	smtpHost := require("INVITO_SMTP_HOST")
	smtpFrom := require("INVITO_SMTP_FROM")
	smtpUser := optional("INVITO_SMTP_USER", "")
	smtpPassword := optional("INVITO_SMTP_PASSWORD", "")

	smtpPortStr := optional("INVITO_SMTP_PORT", "587")
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		errs = append(errs, fmt.Sprintf("INVITO_SMTP_PORT: invalid integer %q", smtpPortStr))
	}

	syncInterval := parseDuration("INVITO_SYNC_INTERVAL", "15m")
	bookingTTL := parseDuration("INVITO_BOOKING_TTL", "24h")

	var key [32]byte
	if secretHex != "" {
		b, err := hex.DecodeString(secretHex)
		if err != nil || len(b) != 32 {
			errs = append(errs, "INVITO_SESSION_SECRET: must be exactly 64 hex characters")
		} else {
			copy(key[:], b)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New("configuration errors:\n  " + strings.Join(errs, "\n  "))
	}

	return &Config{
		BaseURL:          strings.TrimRight(baseURL, "/"),
		DBPath:           optional("INVITO_DB_PATH", "./invito.db"),
		SessionSecret:    key,
		ListenAddr:       optional("INVITO_LISTEN_ADDR", ":8080"),
		OIDCIssuer:       oidcIssuer,
		OIDCClientID:     oidcClientID,
		OIDCClientSecret: oidcClientSecret,
		SMTPHost:         smtpHost,
		SMTPPort:         smtpPort,
		SMTPUser:         smtpUser,
		SMTPPassword:     smtpPassword,
		SMTPFrom:         smtpFrom,
		SyncInterval:     syncInterval,
		BookingTTL:       bookingTTL,
	}, nil
}
