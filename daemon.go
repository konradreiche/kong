package kong

import (
	"encoding/gob"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const refreshRate = 10 * time.Second

// Daemon is an abstraction for the background process which refreshes the Jira
// data. It exists to share access to the Jira client and data between methods.
type Daemon struct {
	jira Jira

	wg   sync.WaitGroup
	data Data
}

// NewDaemon returns a new instance of Daemon.
func NewDaemon(jira Jira) *Daemon {
	return &Daemon{
		jira: jira,
	}
}

// Run executes Kong as background process to periodically fetch Jira data and
// write it to disk for fast retrieval by the CLI.
func (d *Daemon) Run() {
	for {
		if err := d.loop(); err != nil {
			log.Print(err)
		}
		time.Sleep(refreshRate)
	}
}

func (d *Daemon) loop() error {
	// load data in parallel
	loaders := []func(){
		d.loadIssues,
		d.loadSprints,
		d.loadEpics,
		d.loadSprintIssues,
	}
	for _, f := range loaders {
		d.wg.Add(1)
		go f()
	}

	// refresh timestamp
	d.data.Timestamp = time.Now().Unix()

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
			log.Print(err)
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

func (d *Daemon) loadIssues() {
	defer d.wg.Done()
	issues, err := d.jira.ListIssues()
	if err != nil {
		log.Print(err)
		return
	}
	d.data.Issues = issues
}

func (d *Daemon) loadEpics() {
	defer d.wg.Done()
	epics, err := d.jira.ListEpics()
	if err != nil {
		log.Print(err)
		return
	}
	d.data.Epics = epics
}

func (d *Daemon) loadSprintIssues() {
	defer d.wg.Done()
	issues, err := d.jira.ListSprintIssues()
	if err != nil {
		log.Print(err)
		return
	}
	d.data.SprintIssues = issues
}

func (d *Daemon) loadSprints() {
	defer d.wg.Done()
	sprints, err := d.jira.ListSprints()
	if err != nil {
		log.Print(err)
		return
	}
	d.data.Sprints = sprints
	activeSprint, err := sprints.ActiveSprint()
	if err != nil {
		log.Print(err)
	}
	d.data.ActiveSprint = activeSprint
}

func filepath() string {
	return path.Join(os.TempDir(), "kong")
}
