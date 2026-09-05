package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdaptiveGateStartsConservativeAndAIMD(t *testing.T) {
	g := NewAdaptiveGate()
	release, err := g.Acquire(context.Background(), "KR", "ASIA")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = g.Acquire(ctx, "KR", "ASIA")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	release()
	now := time.Now()
	g.ObserveSuccess("ASIA", now)
	require.Equal(t, 1, g.keyLimit["route:ASIA"])
	g.ObserveSuccess("ASIA", now.Add(5*time.Minute))
	require.Equal(t, 2, g.keyLimit["route:ASIA"])
	g.ObserveRateLimit("ASIA", now.Add(5*time.Minute+time.Second))
	require.Equal(t, 1, g.keyLimit["route:ASIA"])
}
