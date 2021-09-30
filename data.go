package kong

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofrs/flock"
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

	path := filepath()
	if data.isMissing() {
		fmt.Fprintln(os.Stderr, "Warning: daemon not running. Performing slow request.")
		return data, nil
	}

	// read file under file lock
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
		fmt.Fprintln(os.Stderr, "Warning: daemon not running. Performing slow request.")
		return data, nil
	}
	return data, nil
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
