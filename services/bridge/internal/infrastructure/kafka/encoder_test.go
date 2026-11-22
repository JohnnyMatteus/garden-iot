package kafka_test

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/sr"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/kafka"
)

func TestEncodesWithConfluentWireFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		switch r.URL.Path {
		case "/subjects/telemetria.bruta.v1-value/versions":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "/schemas/ids/7/versions":
			_, _ = w.Write([]byte(`[{"subject":"telemetria.bruta.v1-value","version":1}]`))
		case "/subjects/telemetria.bruta.v1-value/versions/1":
			_, _ = w.Write([]byte(`{"subject":"telemetria.bruta.v1-value","version":1,"id":7,"schema":"syntax = \"proto3\";","schemaType":"PROTOBUF"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg, err := sr.NewClient(sr.URLs(srv.URL))
	require.NoError(t, err)

	enc, err := kafka.NewProtoEncoder(context.Background(), reg, "telemetria.bruta.v1-value", `syntax = "proto3";`,
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
