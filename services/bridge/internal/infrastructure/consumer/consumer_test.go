package consumer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/sr"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/consumer"
)

func TestDecodeRoundTrip(t *testing.T) {
	at := time.Unix(1732190000, 0)
	body, err := proto.Marshal(&brutav1.TelemetriaBruta{TenantId: "t1", GardenId: "h1", NodeId: "n1", Sequence: 9,
		FwVersion: "1.0.0", ReadAt: timestamppb.New(at), Payload: []byte(`{}`)})
	require.NoError(t, err)
	var h sr.ConfluentHeader
	framed, err := h.AppendEncode(nil, 3, []int{0})
	require.NoError(t, err)

	got, err := consumer.Decode(append(framed, body...))
	require.NoError(t, err)
	assert.Equal(t, "n1", got.Node)
	assert.EqualValues(t, 9, got.Sequence)
	assert.Equal(t, at.Unix(), got.ReadAt.Unix())
}

func TestDecodeRejectsGarbage(t *testing.T) {
	_, err := consumer.Decode([]byte("não é confluent"))
	assert.Error(t, err)
}
