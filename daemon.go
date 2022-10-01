package kong

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"
)

const refreshRate = 10 * time.Second

// Daemon is an abstraction for the background process which refreshes the Jira
// data. It exists to share access to the Jira client and data between methods.
type Daemon struct{}

// NewDaemon returns a new instance of Daemon.
func NewDaemon() (*Daemon, error) {
	if _, err := LoadData(); err != nil {
		return nil, err
	}
	return &Daemon{}, nil
}

// Run executes Kong as background process to periodically fetch Jira data and
// write it to disk for fast retrieval by the CLI.
func (d *Daemon) Run(ctx context.Context) {
	for {
		if err := d.loop(ctx); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
		}
		time.Sleep(refreshRate)
	}
}

func (d *Daemon) loop(ctx context.Context) error {
	data, err := LoadData()
	if err != nil {
		return err
	}
	if err := data.load(ctx); err != nil {
		return err
	}
	// write file under file lock
	return data.writeFile()
}

func filepath() string {
	return path.Join(os.TempDir(), "kong")
}
