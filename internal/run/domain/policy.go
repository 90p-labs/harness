package domain

type Policy struct {
	ApprovalNeeded  bool
	MaxReviewRounds int
	GoLive          bool
	MaxLiveRounds   int
	MaxStopBlocks   int
	StrictMinor     bool
}
