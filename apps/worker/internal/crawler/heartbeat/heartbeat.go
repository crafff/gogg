package heartbeat

import (
	"context"

	"go.temporal.io/sdk/activity"
)

// Record sends a Temporal heartbeat when the crawler phase is running inside
// an Activity. crawler-lite runs the same phase code with a plain context, so
// heartbeat is a no-op there.
func Record(ctx context.Context, details ...any) {
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, details...)
	}
}
