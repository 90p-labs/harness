package domain

import "time"

type Result string

const (
	ResultPass   Result = "pass"
	ResultFail   Result = "fail"
	ResultVetoed Result = "vetoed"
)

type Status string

const (
	StatusPass   Status = "PASS"
	StatusFail   Status = "FAIL"
	StatusVetoed Status = "VETOED"
	StatusCached Status = "CACHED"
)

type Evidence struct {
	Repo      string
	GateID    string
	InputHash string
	Result    Result
	Duration  time.Duration
	Tail      string
	At        time.Time
}
