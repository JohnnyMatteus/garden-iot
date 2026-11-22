// Package httpapi expõe os casos de uso por REST e SSE (contrato em contracts/openapi/garden-api.yaml).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type TokenVerifier interface {
	Verify(token string) (domain.Principal, error)
}

type Server struct {
	Svc       *application.Service
	Verifier  TokenVerifier
	Log       *slog.Logger
	SSEPeriod time.Duration
}

type principalKey struct{}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	r.Group(func(r chi.Router) {
		r.Use(s.authenticate)
		r.Get("/v1/gardens", s.listGardens)
		r.Post("/v1/gardens", s.createGarden)
		r.Route("/v1/gardens/{gardenId}", func(r chi.Router) {
			r.Get("/", s.getGarden)
			r.Get("/overview", s.overview)
			r.Get("/commands", s.listCommands)
			r.Get("/beds", s.listBeds)
			r.Get("/beds/{bedId}/decisions", s.bedDecisions)
			r.Post("/beds", s.createBed)
			r.Get("/nodes", s.listNodes)
			r.Post("/nodes", s.createNode)
			r.Get("/nodes/{nodeId}/readings", s.readings)
			r.Get("/nodes/{nodeId}/readings/aggregate", s.aggregateReadings)
			r.Get("/nodes/{nodeId}/state", s.nodeState)
			r.Post("/nodes/{nodeId}/irrigations", s.requestIrrigation)
			r.Get("/nodes/{nodeId}/commands/{commandId}", s.getCommand)
			r.Get("/alerts", s.listAlerts)
			r.Post("/alerts/{alertId}/ack", s.ackAlert)
		})
		r.Get("/v1/stream/readings", s.streamReadings)
	})
	return r
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == r.Header.Get("Authorization") || token == "" {
			// EventSource não envia cabeçalhos; só o SSE aceita o token na query.
			if r.URL.Path == "/v1/stream/readings" {
				token = r.URL.Query().Get("access_token")
			} else {
				token = ""
			}
		}
		p, err := s.Verifier.Verify(token)
		if err != nil {
			problem(w, http.StatusUnauthorized, "não autenticado", "")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

func principal(r *http.Request) domain.Principal {
	return r.Context().Value(principalKey{}).(domain.Principal)
}

type problemBody struct {
	Title      string          `json:"title"`
	Status     int             `json:"status"`
	Detail     string          `json:"detail,omitempty"`
	Violations []violationBody `json:"violations,omitempty"`
}

type violationBody struct {
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

func problem(w http.ResponseWriter, status int, title, detail string) {
	writeJSON(w, status, "application/problem+json", problemBody{Title: title, Status: status, Detail: detail})
}

func writeJSON(w http.ResponseWriter, status int, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, v any)      { writeJSON(w, http.StatusOK, "application/json", v) }
func created(w http.ResponseWriter, v any) { writeJSON(w, http.StatusCreated, "application/json", v) }

// fail traduz erros de domínio e de aplicação em respostas HTTP.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var pe *domain.PolicyError
	switch {
	case errors.As(err, &pe):
		body := problemBody{Title: "rega bloqueada pela política de segurança", Status: http.StatusConflict}
		for _, v := range pe.Violations {
			body.Violations = append(body.Violations, violationBody{v.Rule, v.Reason})
		}
		writeJSON(w, http.StatusConflict, "application/problem+json", body)
	case errors.Is(err, domain.ErrForbidden):
		problem(w, http.StatusForbidden, "papel sem permissão", "")
	case errors.Is(err, domain.ErrNotFound):
		problem(w, http.StatusNotFound, "não encontrado", "")
	case errors.Is(err, domain.ErrConflict):
		problem(w, http.StatusConflict, "já existe", "")
	case errors.Is(err, domain.ErrInvalidInput):
		problem(w, http.StatusUnprocessableEntity, "entrada inválida", err.Error())
	case errors.Is(err, application.ErrPublish):
		problem(w, http.StatusServiceUnavailable, "broker indisponível", "tente novamente")
	default:
		s.Log.ErrorContext(r.Context(), "erro interno", "err", err, "path", r.URL.Path)
		problem(w, http.StatusInternalServerError, "erro interno", "")
	}
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.Join(domain.ErrInvalidInput, err)
	}
	return nil
}
