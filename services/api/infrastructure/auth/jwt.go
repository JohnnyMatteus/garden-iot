// Package auth valida e emite JWT HS256 com os claims tenant e role.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type claims struct {
	Tenant string `json:"tenant"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type JWT struct {
	secret []byte
	issuer string
}

// NewJWT recusa segredo curto: HS256 com segredo fraco é forjável.
func NewJWT(secret, issuer string) (*JWT, error) {
	if len(secret) < 32 {
		return nil, errors.New("segredo JWT precisa de pelo menos 32 caracteres")
	}
	return &JWT{secret: []byte(secret), issuer: issuer}, nil
}

func (j *JWT) Verify(token string) (domain.Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(j.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return domain.Principal{}, fmt.Errorf("token inválido: %w", err)
	}
	role := domain.Role(c.Role)
	if c.Subject == "" || !domain.ValidIdentifier(c.Tenant) || !role.Valid() {
		return domain.Principal{}, errors.New("token sem sub, tenant ou role válidos")
	}
	return domain.Principal{Subject: c.Subject, TenantID: c.Tenant, Role: role}, nil
}

func (j *JWT) Issue(p domain.Principal, ttl time.Duration, now time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Tenant: p.TenantID, Role: string(p.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: p.Subject, Issuer: j.issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}).SignedString(j.secret)
}
