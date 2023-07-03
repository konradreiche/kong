package kong

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gofrs/flock"
	"golang.org/x/sync/errgroup"
)

const (
	expiry         = refreshRate * 2
	backlogAcronym = "ice"
)

// Data contains all Jira data into one type to easily access any relevant
// information from the CLI but also to serialize and deserialize the data from
// disk.
type Data struct {
	jira Jira

	Timestamp        int64
	Issues           Issues
	IssueByKey       map[string]Issue
	Initiatives      Issues
	Epics            Issues
	SprintIssues     Issues
	BoardID          int
	Sprints          Sprints
	SprintsByName    map[string]Sprint
	ActiveSprint     Sprint
	Transitions      []Transition
	LastIssueCreated string
}

// NewData returns a new instance of Data.
func NewData() Data {
	return Data{
		IssueByKey:    make(map[string]Issue),
		SprintsByName: make(map[string]Sprint),
	}
}

// Stale indicates if the data read from disk is out of date.
func (d Data) Stale() bool {
	timestamp := time.Unix(d.Timestamp, 0)
	return timestamp.Before(time.Now().Add(-expiry))
}

// LoadData parses the Jira state from disk or returns an error if it is out of
// date.
func LoadData() (Data, error) {
	var err error
	data := NewData()

	if data.isMissing() {
		printDaemonWarning()
		if err := data.initJira(); err != nil {
			return data, err
		}
		return data, nil
	}

	// read file under file lock
	path := filepath()
	flock := flock.New(path)
	if err = flock.Lock(); err != nil {
		return data, nil
	}
	defer func() {
		err = flock.Unlock()
	}()

	b, err := os.ReadFile(path)
	if err != nil {
		return data, fmt.Errorf("ReadFile: %w", err)
	}
	decoder := gob.NewDecoder(bytes.NewBuffer(b))
	if err = decoder.Decode(&data); err != nil {
		if err == io.ErrUnexpectedEOF {
			// file corrupt? Deleting
			fmt.Fprintln(os.Stderr, "file potentially corrupt, deleting", path)
			if err := os.Remove(path); err != nil {
				return Data{}, err
			}
			printDaemonWarning()
			return Data{}, nil
		}
		return data, fmt.Errorf("gob.Decode(%s): %w", path, err)
	}

	// report if data is stale but return current data anyway
	if data.Stale() {
		printDaemonWarning()
		if err := data.initJira(); err != nil {
			return data, err
		}
	}
	return data, nil
}

// LoadDataBlocking will fetch all necessary data by synchronously calling the
// Jira API.
//
// This method should be called when all data is needed to perform operations,
// for instance to use the editor when the daemon is not running.
func LoadDataBlocking(ctx context.Context) (Data, error) {
	data, err := LoadData()
	if err != nil {
		return data, err
	}
	return data, data.load(ctx)
}

func (d *Data) load(ctx context.Context) error {
	defer func(startedAt time.Time) {
		fmt.Println("load time", time.Since(startedAt))
	}(time.Now())
	if err := d.initJira(); err != nil {
		return err
	}

	loaders := []func(ctx context.Context) error{
		d.loadIssues,
		d.loadEpics,
		d.loadInitiatives,
		d.loadBoardID,
		d.loadSprintIssues,
		d.loadSprints,
	}

	// load data concurrently
	g, _ := errgroup.WithContext(ctx)
	for _, f := range loaders {
		f := f
		g.Go(func() error {
			return f(ctx)
		})
	}

	// wait until all loaders have finished
	if err := g.Wait(); err != nil {
		return err
	}

	// refresh timestamp
	d.Timestamp = time.Now().Unix()
	return nil
}

func (d *Data) initJira() error {
	jira, err := NewJira()
	if err != nil {
		return err
	}
	d.jira = jira
	return nil
}

func (d *Data) loadIssues(ctx context.Context) error {
	issues, err := d.jira.ListIssues(ctx, d.jira.config.Project)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		d.IssueByKey[issue.Key] = issue
	}
	d.Issues = issues
	return nil
}

func (d *Data) loadEpics(ctx context.Context) error {
	epics, err := d.jira.ListEpics(ctx, d.jira.config.Project)
	if err != nil {
		return err
	}
	d.Epics = epics
	return nil
}

func (d *Data) loadInitiatives(ctx context.Context) error {
	initiatives, err := d.jira.ListInitiatives(ctx, d.jira.config.Project)
	if err != nil {
		return err
	}
	d.Initiatives = initiatives
	return nil
}

func (d *Data) loadBoardID(ctx context.Context) error {
	boardID, err := d.jira.GetBoardID(d.jira.config.Project)
	if err != nil {
		return err
	}
	d.BoardID = boardID
	return nil
}

func (d *Data) loadSprintIssues(ctx context.Context) error {
	issues, err := d.jira.ListSprintIssues(ctx)
	if err != nil {
		return err
	}
	d.SprintIssues = issues
	return nil
}

func (d *Data) loadSprints(ctx context.Context) error {
	if d.BoardID == 0 {
		if err := d.loadBoardID(ctx); err != nil {
			return nil
		}
	}
	sprints, err := d.jira.ListSprints(d.BoardID)
	if err != nil {
		return err
	}
	for _, sprint := range sprints {
		d.SprintsByName[sprint.Name] = sprint
	}
	d.Sprints = sprints
	return nil
}

// GetIssues returns a list of issues. If the data on disk is out of date it
// will request the latest issues from Jira.
func (d Data) GetIssues(ctx context.Context) (Issues, error) {
	if !d.Stale() {
		return d.Issues, nil
	}
	if err := d.loadIssues(ctx); err != nil {
		return nil, err
	}
	return d.Issues, nil
}

// GetEpics returns a list of epic issues. If the data on disk is out of date
// it will request the latest issues from Jira.
func (d Data) GetEpics(ctx context.Context) (Issues, error) {
	if !d.Stale() {
		return d.Epics, nil
	}
	if err := d.loadEpics(ctx); err != nil {
		return nil, err
	}
	return d.Epics, nil
}

// GetInitiatives returns a list of initiative issues. If the data on disk is
// out of date it will request the latest issues from Jira.
func (d Data) GetInitiatives(ctx context.Context) (Issues, error) {
	if !d.Stale() {
		return d.Initiatives, nil
	}
	if err := d.loadInitiatives(ctx); err != nil {
		return nil, err
	}
	return d.Initiatives, nil
}

// GetSprintIssues return a list of issues in the current sprint. If the data
// on disk is out of date it will request the latest issues from Jira.
func (d Data) GetSprintIssues(ctx context.Context) (Issues, error) {
	if !d.Stale() {
		return d.SprintIssues, nil
	}
	if err := d.loadSprintIssues(ctx); err != nil {
		return nil, err
	}
	return d.SprintIssues, nil
}

// GetSprints returns active and future sprints. If the data on disk is out of
// date it will request the latest issues from Jira.
func (d Data) GetSprints(ctx context.Context) (Sprints, error) {
	if !d.Stale() {
		return d.Sprints, nil
	}
	if err := d.loadSprints(ctx); err != nil {
		return nil, err
	}
	return d.Sprints, nil
}

func (d Data) isMissing() bool {
	_, err := os.Stat(filepath())
	return os.IsNotExist(err)
}

func (d Data) WriteFile() error {
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
	err = encoder.Encode(d)
	if err != nil {
		return err
	}
	return file.Close()
}
