package rankings

import "testing"

func TestValidateFilter(t *testing.T) {
	valid := Filter{
		QueueID:           420,
		Version:           "16.13",
		Region:            "KR",
		TierGroup:         "master_plus",
		MinGames:          20,
		PositionThreshold: 5,
		Position:          "MIDDLE",
	}
	if err := ValidateFilter(valid); err != nil {
		t.Fatalf("valid filter rejected: %v", err)
	}

	tests := []struct {
		name string
		mut  func(*Filter)
	}{
		{"queue", func(f *Filter) { f.QueueID = MaxQueueID + 1 }},
		{"min games", func(f *Filter) { f.MinGames = 0 }},
		{"threshold", func(f *Filter) { f.PositionThreshold = 101 }},
		{"version", func(f *Filter) { f.Version = "sixteen" }},
		{"region", func(f *Filter) { f.Region = "EUW1" }},
		{"tier", func(f *Filter) { f.TierGroup = "diamond" }},
		{"position", func(f *Filter) { f.Position = "CENTER" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := valid
			tc.mut(&got)
			if err := ValidateFilter(got); err == nil {
				t.Fatal("invalid filter accepted")
			}
		})
	}
}
