package kong

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"os"
	"time"

	"github.com/gofrs/flock"
	"golang.org/x/sync/errgroup"
)

const expiry = refreshRate * 2

// Data contains all Jira data into one type to easily access any relevant
// information from the CLI but also to serialize and deserialize the data from
// disk.
type Data struct {
	Timestamp int64
	Issues    Issues
	Epics     Issues
	Sprints   Sprints
}

// Stale indicates if the data read from disk is out of date.
func (d Data) Stale() bool {
	timestamp := time.Unix(d.Timestamp, 0)
	return timestamp.Before(time.Now().Add(-expiry))
}

// LoadData parses the Jira state from disk or returns an error if it is out
// of date.
func LoadData() (Data, error) {
	var data Data
	if _, err := LoadConfig(); err != nil {
		return data, err
	}

	if data.isMissing() {
		printDaemonWarning()
		return data, nil
	}

	// read file under file lock
	path := filepath()
	flock := flock.New(path)
	err := flock.Lock()
	if err != nil {
		return data, err
	}
	defer func() {
		err = flock.Unlock()
		if err != nil {
			log.Print(err)
		}
	}()

	b, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}
	decoder := gob.NewDecoder(bytes.NewBuffer(b))
	err = decoder.Decode(&data)
	if err != nil {
		printDaemonWarning()
		return data, nil
	}

	// report if data is stale but return current data anyway
	if data.Stale() {
		printDaemonWarning()
	}
	return data, nil
}

// LoadDataBlocking will fetch all necessary data by synchronously calling the
// Jira API.
//
// This method should be called when all data is needed to perform operations,
// for instance to use the editor when the daemon is not running.
func LoadDataBlocking(ctx context.Context) (Data, error) {
	var data Data
	jira, err := NewJira()
	if err != nil {
		return data, err
	}

	// fetch data concurrently but return an error if any of them fails
	g, _ := errgroup.WithContext(ctx)

	// get issues
	g.Go(func() error {
		issues, err := jira.ListIssues()
		if err != nil {
			return err
		}
		data.Issues = issues
		return nil
	})

	// get issues
	g.Go(func() error {
		epics, err := jira.ListEpics()
		if err != nil {
			return err
		}
		data.Epics = epics
		return nil
	})

	// get sprints
	g.Go(func() error {
		sprints, err := jira.ListSprints()
		if err != nil {
			return err
		}
		data.Sprints = sprints
		return nil
	})

	return data, g.Wait()
}

// GetIssues returns a list of issues. If the data on disk is out of date it
// will request the latest issues from Jira.
func (d Data) GetIssues() (Issues, error) {
	if d.Stale() {
		jira, err := NewJira()
		if err != nil {
			return nil, err
		}
		return jira.ListIssues()
	}
	return d.Issues, nil
}

// GetEpics returns a list of epic issues. If the data on disk is out of date
// it will request the latest issues from Jira.
func (d Data) GetEpics() (Issues, error) {
	if d.Stale() {
		jira, err := NewJira()
		if err != nil {
			return nil, err
		}
		return jira.ListEpics()
	}
	return d.Epics, nil
}

// GetSprints returns active and future sprints. If the data on disk is out of
// date it will request the latest issues from Jira.
func (d Data) GetSprints() (Sprints, error) {
	if d.Stale() {
		jira, err := NewJira()
		if err != nil {
			return nil, err
		}
		return jira.ListSprints()
	}
	return d.Sprints, nil
}

func (d Data) isMissing() bool {
	_, err := os.Stat(filepath())
	return os.IsNotExist(err)
}
