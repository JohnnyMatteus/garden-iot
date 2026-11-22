package metrics_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/metrics"
)

func TestExposesInvalidByReason(t *testing.T) {
	p := metrics.NewPrometheus()
	p.Invalid(domain.ReasonDecode)
	p.Forwarded("t1")

	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	assert.Contains(t, rec.Body.String(), `bridge_messages_invalid_total{reason="payload_invalido"} 1`)
	assert.Contains(t, rec.Body.String(), `bridge_messages_forwarded_total{tenant="t1"} 1`)
}
