package kong

import (
	"errors"
)

// ErrConfigMissing is returned when the configuration file does not exist.
var ErrConfigMissing = errors.New("configuration missing")

// ErrDataMissing is returned if the data file created by the daemon does not
// exist.
var ErrDataMissing = errors.New("data file missing")

// ErrCreateSprint is used to wrap the Jira API response returned on sprint
// creation failure.
type ErrCreateSprint string

func (e ErrCreateSprint) Error() string {
	return string(e)
}
