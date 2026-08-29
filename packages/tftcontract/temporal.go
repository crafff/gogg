package tftcontract

import "time"

const (
	CrawlWorkflowName        = "TFTCrawlWorkflow"
	PlatformWorkflowName     = "TFTPlatformSeedWorkflow"
	RouteWorkflowName        = "TFTRouteDispatchWorkflow"
	StaticWorkflowName       = "TFTStaticSyncWorkflow"
	PlayerLookupWorkflowName = "TFTPlayerLookupWorkflow"
	CrawlSignalName          = "tft-control"
	CrawlStatusQueryName     = "tft-status"
	StaticStatusQueryName    = "tft-static-status"
	SeedTaskQueue            = "tft-seed"
	StaticTaskQueue          = "tft-static"
	DefaultScheduleID        = "gogg-tft-crawl-global"
	DefaultStaticScheduleID  = "gogg-tft-static"
)

func MatchTaskQueue(routingRegion string) string {
	switch routingRegion {
	case "AMERICAS":
		return "tft-match-americas"
	case "ASIA":
		return "tft-match-asia"
	case "EUROPE":
		return "tft-match-europe"
	case "SEA":
		return "tft-match-sea"
	default:
		return "tft-match-unknown"
	}
}

type CrawlInput struct {
	ProfileName string
	Platforms   []string
	Patch       string
	Set         string
	WindowStart time.Time
	WindowEnd   time.Time
	Window      time.Duration
	WindowLag   time.Duration
}

type PlatformInput struct {
	CrawlInput CrawlInput
	RunID      int64
	Platform   string
}

type RouteInput struct {
	RunID         int64
	RoutingRegion string
	Patch         string
}

type PlayerLookupInput struct {
	JobID, Platform, GameName, TagLine string
}

type ControlCommand struct {
	Action string
}

type CrawlStatus struct {
	State          string
	Stage          string
	RunID          int64
	PausePending   bool
	StageCompleted int
	StageTotal     int
}

type StaticStatus struct {
	State, Stage, Source string
	SnapshotIDs          []int64
	Total                int64
	Completed            int64
	Skipped              int64
	Failed               int64
	Remaining            int64
	Fetched              int64
}
