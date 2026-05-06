package auth

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/jeboehm/invito/internal/config"
)

type Claims struct {
	Sub               string
	Email             string
	Name              string
	PreferredUsername string
}

type Provider struct {
	provider  *gooidc.Provider
	verifier  *gooidc.IDTokenVerifier
	oauth2cfg oauth2.Config
}

func NewProvider(ctx context.Context, cfg *config.Config) (*Provider, error) {
	p, err := gooidc.NewProvider(ctx, cfg.OIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}

	oauth2cfg := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  cfg.BaseURL + "/auth/callback",
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}

	verifier := p.Verifier(&gooidc.Config{ClientID: cfg.OIDCClientID})

	return &Provider{
		provider:  p,
		verifier:  verifier,
		oauth2cfg: oauth2cfg,
	}, nil
}

func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth2cfg.AuthCodeURL(state)
}

func (p *Provider) Exchange(ctx context.Context, code string) (*Claims, error) {
	token, err := p.oauth2cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var raw struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	return &Claims{
		Sub:               raw.Sub,
		Email:             raw.Email,
		Name:              raw.Name,
		PreferredUsername: raw.PreferredUsername,
	}, nil
}

// SlugifyUsername converts a preferred_username claim to a URL-safe slug.
func SlugifyUsername(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	slug := b.String()
	if len(slug) == 0 {
		return "user"
	}
	return slug
}
