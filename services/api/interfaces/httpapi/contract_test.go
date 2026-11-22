package httpapi_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"log/slog"

	"github.com/JohnnyMatteus/garden-iot/services/api/interfaces/httpapi"
)

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "..", "..", "contracts", "openapi", "garden-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI inválido: %v", err)
	}
	return doc
}

// Toda rota do servidor precisa estar documentada, e toda operação documentada precisa existir.
func TestRoutesMatchOpenAPI(t *testing.T) {
	doc := loadSpec(t)
	documented := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			documented[method+" "+path] = true
		}
	}
	srv := &httpapi.Server{Log: slog.Default()}
	implemented := map[string]bool{}
	err := chi.Walk(srv.Handler().(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		if route != "" {
			implemented[method+" "+route] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k := range implemented {
		if !documented[k] {
			t.Errorf("rota sem documentação no OpenAPI: %s", k)
		}
	}
	for k := range documented {
		if !implemented[k] {
			t.Errorf("operação documentada e não implementada: %s", k)
		}
	}
}
