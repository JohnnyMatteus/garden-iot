package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/auth"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestRoundTrip(t *testing.T) {
	j, _ := auth.NewJWT(secret, "garden-api")
	want := domain.Principal{Subject: "ana", TenantID: "demo", Role: domain.RoleOperator}
	tok, err := j.Issue(want, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := j.Verify(tok)
	if err != nil || got != want {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestRejects(t *testing.T) {
	j, _ := auth.NewJWT(secret, "garden-api")
	other, _ := auth.NewJWT(strings.Repeat("x", 32), "garden-api")
	wrongIssuer, _ := auth.NewJWT(secret, "outro")
	p := domain.Principal{Subject: "ana", TenantID: "demo", Role: domain.RoleOwner}
	expired, _ := j.Issue(p, time.Minute, time.Now().Add(-time.Hour))
	forged, _ := other.Issue(p, time.Hour, time.Now())
	foreign, _ := wrongIssuer.Issue(p, time.Hour, time.Now())
	badRole, _ := j.Issue(domain.Principal{Subject: "ana", TenantID: "demo", Role: "root"}, time.Hour, time.Now())
	badTenant, _ := j.Issue(domain.Principal{Subject: "ana", TenantID: "DEMO; DROP", Role: domain.RoleOwner}, time.Hour, time.Now())
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": "a", "tenant": "demo", "role": "owner", "iss": "garden-api", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	for name, tok := range map[string]string{"expirado": expired, "assinatura errada": forged, "emissor errado": foreign,
		"papel inválido": badRole, "tenant inválido": badTenant, "alg none": none, "lixo": "abc"} {
		if _, err := j.Verify(tok); err == nil {
			t.Errorf("%s: deveria ser rejeitado", name)
		}
	}
}

func TestShortSecretRefused(t *testing.T) {
	if _, err := auth.NewJWT("curto", "x"); err == nil {
		t.Fatal("segredo curto deveria falhar")
	}
}
