package quantity_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/JohnnyMatteus/garden-iot/contracts/quantity"
)

var kindRef = regexp.MustCompile(`kind\s*=\s*'([^']+)'`)

func known(t *testing.T, name, where string) {
	t.Helper()
	if !slices.Contains(quantity.All, name) {
		t.Errorf("%s usa grandeza %q fora de contracts/quantity", where, name)
	}
}

// Painéis do Grafana consultam `kind` direto no SQL: nome divergente devolve série vazia sem erro.
func TestGrafanaDashboardsUseContractNames(t *testing.T) {
	files, err := filepath.Glob("../../infra/grafana/dashboards/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("painéis não encontrados: %v", err)
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for _, m := range kindRef.FindAllStringSubmatch(string(b), -1) {
			known(t, m[1], f)
		}
	}
}

// O README do contrato MQTT documenta a tabela de grandezas normalizadas.
func TestMQTTReadmeListsAllQuantities(t *testing.T) {
	b, err := os.ReadFile("../mqtt/README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range quantity.All {
		if !regexp.MustCompile("`" + regexp.QuoteMeta(q) + "`").Match(b) {
			t.Errorf("README do contrato MQTT não documenta %q", q)
		}
	}
}

// Só o parâmetro de consulta e o schema Reading falam de grandeza; o `kind` de Alert é outro conceito.
func TestOpenAPIExamplesUseContractNames(t *testing.T) {
	b, err := os.ReadFile("../openapi/garden-api.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ref := regexp.MustCompile(`(?:name: kind, in: query.*|Reading:[\s\S]*?kind: \{[^}]*)example: (\w+)`)
	m := ref.FindAllStringSubmatch(string(b), -1)
	if len(m) < 2 {
		t.Fatalf("exemplos de grandeza não encontrados (%d)", len(m))
	}
	for _, x := range m {
		known(t, x[1], "openapi")
	}
}
