package kafka_test

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/sr"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/kafka"
)

func TestEncodesWithConfluentWireFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		switch r.URL.Path {
		case "/subjects/telemetria.bruta.v1-value/versions/latest":
			_, _ = w.Write([]byte(`{"subject":"telemetria.bruta.v1-value","version":1,"id":7,"schema":"syntax = \"proto3\";","schemaType":"PROTOBUF"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg, err := sr.NewClient(sr.URLs(srv.URL))
	require.NoError(t, err)

	enc, err := kafka.NewProtoEncoder(context.Background(), reg, "telemetria.bruta.v1-value",
		&wrapperspb.StringValue{}, func(tm domain.Telemetry) proto.Message { return wrapperspb.String(tm.NoID) })
	require.NoError(t, err)

	out, err := enc.Encode(domain.Telemetry{NoID: "N"})
	require.NoError(t, err)

	assert.Equal(t, byte(0), out[0], "magic byte")
	assert.EqualValues(t, 7, binary.BigEndian.Uint32(out[1:5]), "id do schema")
	assert.Equal(t, byte(0), out[5], "índice da mensagem")

	var got wrapperspb.StringValue
	require.NoError(t, proto.Unmarshal(out[6:], &got))
	assert.Equal(t, "N", got.Value)
}

func TestToBrutaMapsAllFields(t *testing.T) {
	at := time.Unix(1763726400, 0)
	m := kafka.ToBruta(domain.Telemetry{Tenant: "t1", Horta: "h1", NoID: "N", Sequence: 10, FwVersion: "1.0.0",
		ReadAt: at, ReceivedAt: at.Add(time.Second), Payload: []byte("x")}).(*brutav1.TelemetriaBruta)
	assert.Equal(t, "N", m.NodeId)
	assert.EqualValues(t, 10, m.Sequence)
	assert.Equal(t, at.Unix(), m.ReadAt.Seconds)
	assert.Equal(t, []byte("x"), m.Payload)
}
