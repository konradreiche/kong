package kong

import (
	"errors"
	"fmt"
	"os"
)

// ErrConfigMissing is returned when the configuration file does not exist.
var ErrConfigMissing = errors.New("configuration missing")

// ErrDataMissing is returned if the data file created by the daemon does not
// exist.
var ErrDataMissing = errors.New("data file missing")

// ErrNoActiveSprint is returned when there is no active sprint in a list of
// sprints.
var ErrNoActiveSprint = errors.New("no active sprint")

// ErrCreateSprint is used to wrap the Jira API response returned on sprint
// creation failure.
type ErrCreateSprint string

func (e ErrCreateSprint) Error() string {
	return string(e)
}

func printDaemonWarning() {
	fmt.Fprintln(os.Stderr, "Warning: daemon not running, check logs. Performing slow request.")
}
