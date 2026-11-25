// Copyright Contributors to the Open Cluster Management project

package version

import "fmt"

var (
	Version   = "0.2.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func String() string {
	return fmt.Sprintf("version=%s commit=%s built=%s", Version, GitCommit, BuildDate)
}
