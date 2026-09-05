//go:build integration

package workflow

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	temporalworkflow "go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/packages/tftcontract"
)

func TestReplayLiveTFTCrawlHistory(t *testing.T) {
	workflowID := os.Getenv("GOGG_TFT_REPLAY_WORKFLOW_ID")
	runID := os.Getenv("GOGG_TFT_REPLAY_RUN_ID")
	if workflowID == "" || runID == "" {
		t.Skip("set GOGG_TFT_REPLAY_WORKFLOW_ID and GOGG_TFT_REPLAY_RUN_ID")
	}
	hostPort := os.Getenv("GOGG_TEMPORAL_HOST_PORT")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}
	namespace := os.Getenv("GOGG_TEMPORAL_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	temporalClient, err := client.Dial(client.Options{HostPort: hostPort, Namespace: namespace})
	require.NoError(t, err)
	defer temporalClient.Close()

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflow(Crawl)
	replayer.RegisterWorkflowWithOptions(Crawl, temporalworkflow.RegisterOptions{Name: tftcontract.CrawlWorkflowName})
	require.NoError(t, replayer.ReplayWorkflowExecution(
		context.Background(), temporalClient.WorkflowService(), nil, namespace,
		temporalworkflow.Execution{ID: workflowID, RunID: runID},
	))
}
