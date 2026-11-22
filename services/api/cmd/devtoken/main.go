// Comando devtoken: emite JWT de desenvolvimento. Uso: devtoken <tenant> <owner|operator|viewer> [subject]
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/auth"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: devtoken <tenant> <owner|operator|viewer> [subject]")
		os.Exit(2)
	}
	issuer := os.Getenv("API_JWT_ISSUER")
	if issuer == "" {
		issuer = "garden-api"
	}
	j, err := auth.NewJWT(os.Getenv("API_JWT_SECRET"), issuer)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	subject := "dev-user"
	if len(os.Args) > 3 {
		subject = os.Args[3]
	}
	tok, err := j.Issue(domain.Principal{Subject: subject, TenantID: os.Args[1], Role: domain.Role(os.Args[2])}, 8*time.Hour, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(tok)
}
