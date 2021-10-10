package kong

import (
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/gofrs/flock"
)

const refreshRate = 10 * time.Second

// Daemon is an abstraction for the background process which refreshes the Jira
// data. It exists to share access to the Jira client and data between methods.
type Daemon struct {
	data Data
}

// NewDaemon returns a new instance of Daemon.
func NewDaemon() (*Daemon, error) {
	data, err := LoadData()
	if err != nil {
		return nil, err
	}
	return &Daemon{
		data: data,
	}, nil
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
	if err := d.data.load(ctx); err != nil {
		return err
	}
	// write file under file lock
	return d.writeFile()
}

func (d *Daemon) writeFile() error {
	path := filepath()
	flock := flock.New(path)
	err := flock.Lock()
	if err != nil {
		return err
	}
	defer func() {
		err = flock.Unlock()
		if err != nil {
			fmt.Fprint(os.Stderr, err)
		}
	}()
	// create or overwrite file at /tmp/kong
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(d.data)
	if err != nil {
		return err
	}
	return file.Close()
}

func filepath() string {
	return path.Join(os.TempDir(), "kong")
}
