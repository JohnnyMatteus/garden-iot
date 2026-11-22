package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/auth"
	"github.com/JohnnyMatteus/garden-iot/services/api/interfaces/httpapi"
)

const secret = "0123456789abcdef0123456789abcdef"

type catalog struct{ application.Catalog }

func (catalog) GetNode(_ context.Context, tenant, g, n string) (domain.Node, error) {
	if tenant != "demo" || n != "n3" {
		return domain.Node{}, domain.ErrNotFound
	}
	return domain.Node{ID: "n3", GardenID: g, Kind: domain.ActuatorNode}, nil
}
func (catalog) GetGarden(_ context.Context, tenant, g string) (domain.Garden, error) {
	if tenant != "demo" {
		return domain.Garden{}, domain.ErrNotFound
	}
	return domain.Garden{ID: g, Name: "Horta"}, nil
}
func (catalog) ListGardens(_ context.Context, tenant string) ([]domain.Garden, error) {
	return []domain.Garden{{ID: "horta-1", Name: tenant}}, nil
}

type readings struct {
	application.ReadingStore
	temp *float64
	feed []application.IngestedReading
}

func (r readings) Latest(_ context.Context, _, _, kind string, _ time.Time) (*domain.Reading, error) {
	if kind == "air_temp_c" && r.temp != nil {
		return &domain.Reading{Value: *r.temp}, nil
	}
	return nil, nil
}
func (r readings) IngestedAfter(context.Context, string, string, time.Time, int) ([]application.IngestedReading, error) {
	return r.feed, nil
}

type twin struct{ application.TwinStore }

func (twin) SetDesired(context.Context, string, string, string, domain.ActuatorState) error {
	return nil
}

type commands struct{ application.CommandStore }

func (commands) HasOpen(context.Context, string, string, string) (bool, error)         { return false, nil }
func (commands) Create(context.Context, string, domain.Command) error                  { return nil }
func (commands) SetStatus(context.Context, string, string, domain.CommandStatus) error { return nil }

type publisher struct{ n int }

func (p *publisher) Publish(context.Context, string, domain.Command) error { p.n++; return nil }

type clock struct{}

func (clock) Now() time.Time { return time.Now() }

type ids struct{}

func (ids) NewID() string { return "11111111-1111-1111-1111-111111111111" }

func setup(t *testing.T, temp *float64, feed []application.IngestedReading) (*httptest.Server, *auth.JWT, *publisher) {
	t.Helper()
	jwtAuth, _ := auth.NewJWT(secret, "garden-api")
	pub := &publisher{}
	svc := &application.Service{Catalog: catalog{}, Readings: readings{temp: temp, feed: feed}, Twin: twin{}, Commands: commands{},
		Publisher: pub, Policy: domain.DefaultIrrigationPolicy(), Clock: clock{}, IDs: ids{},
		CommandTTL: 30 * time.Second, ConditionsMaxAge: time.Hour}
	srv := httptest.NewServer((&httpapi.Server{Svc: svc, Verifier: jwtAuth, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), SSEPeriod: 20 * time.Millisecond}).Handler())
	t.Cleanup(srv.Close)
	return srv, jwtAuth, pub
}

func token(t *testing.T, j *auth.JWT, tenant string, role domain.Role) string {
	tok, err := j.Issue(domain.Principal{Subject: "ana", TenantID: tenant, Role: role}, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func do(t *testing.T, method, url, tok, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

const irrigationPath = "/v1/gardens/horta-1/nodes/n3/irrigations"

func TestUnauthenticatedIsRejected(t *testing.T) {
	srv, _, _ := setup(t, nil, nil)
	if resp, _ := do(t, "GET", srv.URL+"/v1/gardens", "", ""); resp.StatusCode != 401 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp, _ := do(t, "GET", srv.URL+"/v1/gardens?access_token=x", "", ""); resp.StatusCode != 401 {
		t.Fatalf("token na query só vale para o SSE; status = %d", resp.StatusCode)
	}
}

func TestListGardensUsesTenantFromToken(t *testing.T) {
	srv, j, _ := setup(t, nil, nil)
	resp, body := do(t, "GET", srv.URL+"/v1/gardens", token(t, j, "demo", domain.RoleViewer), "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"name":"demo"`) {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestIrrigationAccepted(t *testing.T) {
	srv, j, pub := setup(t, nil, nil)
	resp, body := do(t, "POST", srv.URL+irrigationPath, token(t, j, "demo", domain.RoleOperator), `{"durationSeconds":120}`)
	if resp.StatusCode != 202 {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	var c struct{ Status string }
	_ = json.Unmarshal([]byte(body), &c)
	if c.Status != "sent" || pub.n != 1 {
		t.Fatalf("status %q, publicações %d", c.Status, pub.n)
	}
}

func TestIrrigationViewerForbidden(t *testing.T) {
	srv, j, pub := setup(t, nil, nil)
	resp, _ := do(t, "POST", srv.URL+irrigationPath, token(t, j, "demo", domain.RoleViewer), `{"durationSeconds":120}`)
	if resp.StatusCode != 403 || pub.n != 0 {
		t.Fatalf("status %d, publicações %d", resp.StatusCode, pub.n)
	}
}

func TestIrrigationBlockedByFrostReturns409WithViolations(t *testing.T) {
	cold := 0.5
	srv, j, pub := setup(t, &cold, nil)
	resp, body := do(t, "POST", srv.URL+irrigationPath, token(t, j, "demo", domain.RoleOwner), `{"durationSeconds":120}`)
	if resp.StatusCode != 409 || !strings.Contains(body, `"rule":"frost"`) || pub.n != 0 {
		t.Fatalf("status %d body %s publicações %d", resp.StatusCode, body, pub.n)
	}
}

func TestIrrigationOtherTenantSeesNotFound(t *testing.T) {
	srv, j, _ := setup(t, nil, nil)
	resp, _ := do(t, "POST", srv.URL+irrigationPath, token(t, j, "outro", domain.RoleOwner), `{"durationSeconds":120}`)
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestIrrigationRejectsUnknownFields(t *testing.T) {
	srv, j, _ := setup(t, nil, nil)
	resp, _ := do(t, "POST", srv.URL+irrigationPath, token(t, j, "demo", domain.RoleOwner), `{"durationSeconds":120,"tenant":"outro"}`)
	if resp.StatusCode != 422 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSSEStreamsReadingsAndAcceptsQueryToken(t *testing.T) {
	feed := []application.IngestedReading{{
		Reading: domain.Reading{NodeID: "n1", SensorID: "soil-1", Kind: "soil_moisture_vwc_pct", Value: 41, Unit: "%", Quality: "ok"}, IngestedAt: time.Now()}}
	srv, j, _ := setup(t, nil, feed)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/v1/stream/readings?garden=horta-1&access_token="+token(t, j, "demo", domain.RoleViewer), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	if got := string(buf[:n]); !strings.Contains(got, "event: reading") || !strings.Contains(got, `"kind":"soil_moisture_vwc_pct"`) {
		t.Fatalf("evento inesperado: %q", got)
	}
}
