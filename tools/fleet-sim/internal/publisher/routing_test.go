package publisher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutingErrorOnlyForTelemetryWithoutSubscribers(t *testing.T) {
	assert.ErrorIs(t, routingError("horta/t1/h1/n1/telemetria", 0x10), ErrNoMatchingSubscribers)
	assert.NoError(t, routingError("horta/t1/h1/n1/telemetria", 0x00))
	assert.NoError(t, routingError("horta/t1/h1/n1/estado", 0x10), "estado pode não ter assinante")
	assert.NoError(t, routingError("horta/t1/h1/n1/lwt", 0x10), "lwt pode não ter assinante")
}
