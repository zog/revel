package jobs

import (
	"github.com/robfig/revel/jobs"
)

// This job checks nightly that hotels have not been overbooked.
func checkStuff() {
}

func init() {
	jobs.OnAppStart(checkStuff)
}
