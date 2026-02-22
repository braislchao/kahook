package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func String() string {
	return fmt.Sprintf("Kahook %s (commit: %s, built: %s, %s)",
		Version, GitCommit, BuildTime, runtime.Version())
}
