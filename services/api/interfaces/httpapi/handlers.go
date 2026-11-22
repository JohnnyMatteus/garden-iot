package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

func (s *Server) listGardens(w http.ResponseWriter, r *http.Request) {
	gs, err := s.Svc.Catalog.ListGardens(r.Context(), principal(r).TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(gs, toGarden))
}

func (s *Server) createGarden(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID, Name string }
	if err := decode(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	g, err := s.Svc.CreateGarden(r.Context(), principal(r), domain.Garden{ID: in.ID, Name: in.Name})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	created(w, toGarden(g))
}

func (s *Server) getGarden(w http.ResponseWriter, r *http.Request) {
	g, err := s.Svc.Catalog.GetGarden(r.Context(), principal(r).TenantID, chi.URLParam(r, "gardenId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, toGarden(g))
}

func (s *Server) listBeds(w http.ResponseWriter, r *http.Request) {
	bs, err := s.Svc.Catalog.ListBeds(r.Context(), principal(r).TenantID, chi.URLParam(r, "gardenId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(bs, toBed))
}

func (s *Server) createBed(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID, Name, Crop, Irrigation string
	}
	if err := decode(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	b, err := s.Svc.CreateBed(r.Context(), principal(r), domain.Bed{
		ID: in.ID, GardenID: chi.URLParam(r, "gardenId"), Name: in.Name, Crop: in.Crop, Irrigation: domain.IrrigationType(in.Irrigation)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	created(w, toBed(b))
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	ns, err := s.Svc.Catalog.ListNodes(r.Context(), principal(r).TenantID, chi.URLParam(r, "gardenId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(ns, toNode))
}

func (s *Server) createNode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID, Kind string
		BedID    *string
	}
	if err := decode(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	n, err := s.Svc.CreateNode(r.Context(), principal(r), domain.Node{
		ID: in.ID, GardenID: chi.URLParam(r, "gardenId"), Kind: domain.NodeKind(in.Kind), BedID: in.BedID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	created(w, toNode(n))
}

func parseTime(r *http.Request, name string) (time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return t, fmt.Errorf("%w: %s precisa ser RFC 3339", domain.ErrInvalidInput, name)
	}
	return t, nil
}

func (s *Server) readings(w http.ResponseWriter, r *http.Request) {
	q := application.ReadingQuery{GardenID: chi.URLParam(r, "gardenId"), NodeID: chi.URLParam(r, "nodeId"), Kind: r.URL.Query().Get("kind")}
	var err error
	if q.From, err = parseTime(r, "from"); err == nil {
		q.To, err = parseTime(r, "to")
	}
	if err == nil {
		if l := r.URL.Query().Get("limit"); l != "" {
			if q.Limit, err = strconv.Atoi(l); err != nil {
				err = fmt.Errorf("%w: limit", domain.ErrInvalidInput)
			}
		}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rs, err := s.Svc.History(r.Context(), principal(r), q)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(rs, toReading))
}

func (s *Server) nodeState(w http.ResponseWriter, r *http.Request) {
	st, err := s.Svc.NodeState(r.Context(), principal(r), chi.URLParam(r, "gardenId"), chi.URLParam(r, "nodeId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, toState(st))
}

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	open, _ := strconv.ParseBool(r.URL.Query().Get("open"))
	as, err := s.Svc.Alerts.List(r.Context(), principal(r).TenantID, chi.URLParam(r, "gardenId"), open)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(as, toAlert))
}

func (s *Server) ackAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.Svc.AcknowledgeAlert(r.Context(), principal(r), chi.URLParam(r, "gardenId"), chi.URLParam(r, "alertId")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requestIrrigation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DurationSeconds int `json:"durationSeconds"`
	}
	if err := decode(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	cmd, err := s.Svc.RequestIrrigation(r.Context(), principal(r), chi.URLParam(r, "gardenId"), chi.URLParam(r, "nodeId"),
		time.Duration(in.DurationSeconds)*time.Second)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, "application/json", toCommand(cmd))
}

func (s *Server) getCommand(w http.ResponseWriter, r *http.Request) {
	c, err := s.Svc.Commands.Get(r.Context(), principal(r).TenantID, chi.URLParam(r, "gardenId"), chi.URLParam(r, "nodeId"), chi.URLParam(r, "commandId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, toCommand(c))
}

func (s *Server) streamReadings(w http.ResponseWriter, r *http.Request) {
	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		problem(w, http.StatusInternalServerError, "streaming não suportado", "")
		return
	}
	ch, err := s.Svc.StreamReadings(r.Context(), principal(r), r.URL.Query().Get("garden"), s.SSEPeriod)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case rd, open := <-ch:
			if !open {
				return
			}
			b, _ := json.Marshal(toReading(rd))
			fmt.Fprintf(w, "event: reading\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	gardenID := chi.URLParam(r, "gardenId")
	o, err := s.Svc.Overview(r.Context(), principal(r), gardenID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, overviewDTO{GardenID: gardenID, GeneratedAt: s.Svc.Clock.Now(), OpenAlerts: o.OpenAlerts, Beds: mapList(o.Beds, toBedOverview)})
}

func queryInt(r *http.Request, name string) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", domain.ErrInvalidInput, name)
	}
	return n, nil
}

func (s *Server) listCommands(w http.ResponseWriter, r *http.Request) {
	limit, err := queryInt(r, "limit")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cs, err := s.Svc.ListCommands(r.Context(), principal(r), chi.URLParam(r, "gardenId"), r.URL.Query().Get("nodeId"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(cs, toCommand))
}

func (s *Server) bedDecisions(w http.ResponseWriter, r *http.Request) {
	limit, err := queryInt(r, "limit")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ds, err := s.Svc.BedDecisions(r.Context(), principal(r), chi.URLParam(r, "gardenId"), chi.URLParam(r, "bedId"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(ds, toDecision))
}

func (s *Server) aggregateReadings(w http.ResponseWriter, r *http.Request) {
	bucket, err := time.ParseDuration(r.URL.Query().Get("bucket"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: bucket (ex.: 5m, 1h, 24h)", domain.ErrInvalidInput))
		return
	}
	q := application.AggregateQuery{GardenID: chi.URLParam(r, "gardenId"), NodeID: chi.URLParam(r, "nodeId"), Kind: r.URL.Query().Get("kind"), Bucket: bucket}
	if q.From, err = parseTime(r, "from"); err == nil {
		q.To, err = parseTime(r, "to")
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ps, err := s.Svc.Aggregate(r.Context(), principal(r), q)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ok(w, mapList(ps, toAggregatePoint))
}
