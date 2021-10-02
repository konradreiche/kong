package kong

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/trivago/tgo/tcontainer"
)

var (
	errMissingColumn  = errors.New("missing column")
	errEpicMismatch   = errors.New("epic does not exist")
	errSprintMismatch = errors.New("sprint does not exist")
)

// Editor provides any functionality that processes user input by providing a
// editor which creates files and parses back the content the user provided.
type Editor struct {
	jira   Jira
	data   Data
	config Config
}

// NewEditor returns a new instace of Editor.
func NewEditor() (Editor, error) {
	jira, err := NewJira()
	if err != nil {
		return Editor{}, err
	}
	data, err := LoadData()
	if err != nil {
		return Editor{}, err
	}
	return Editor{
		jira:   jira,
		data:   data,
		config: jira.config,
	}, nil
}

// OpenIssueEditor creates a new file create Jira issues in batches.
func (e Editor) OpenIssueEditor(ctx context.Context) error {
	template := e.issueTemplate()
	f, err := os.CreateTemp(os.TempDir(), "kong-new-issues")
	if err != nil {
		return err
	}
	_, err = f.WriteString(template)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			log.Print(err)
		}
	}()

	for {
		args := []string{f.Name(), "-c", "norm! G"}
		cmd := exec.CommandContext(ctx, "vim", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(f.Name())
		if err != nil {
			return err
		}
		lines := e.parseLines(string(b))

		// abort on empty input
		if len(lines) == 0 {
			return nil
		}

		issues, err := e.parseIssues(lines)
		if err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}
		return e.jira.CreateIssues(ctx, issues)
	}
}

func (e Editor) parseLines(s string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "#") && line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (e Editor) parseIssues(lines []string) ([]*jira.Issue, error) {
	issues := make([]*jira.Issue, 0)
	for _, line := range lines {
		columns := strings.Split(line, ",")
		if len(columns) != 5 {
			return nil, errMissingColumn
		}
		issue, err := e.parseIssue(columns)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (e Editor) parseIssue(columns []string) (*jira.Issue, error) {
	epicIndex, err := strconv.Atoi(columns[0])
	if err != nil {
		return nil, err
	}
	sprintIndex, err := strconv.Atoi(columns[1])
	if err != nil {
		return nil, err
	}

	summary := columns[2]

	storyPoints, err := strconv.ParseFloat(columns[3], 64)
	if err != nil {
		return nil, err
	}

	description := columns[4]

	// verify epic index matches available epics
	if epicIndex < 0 || epicIndex >= len(e.data.Epics) {
		return nil, errEpicMismatch
	}

	// verify sprint index matches available sprints
	if sprintIndex < 0 || sprintIndex > len(e.data.Sprints) {
		return nil, errSprintMismatch
	}

	epic := e.data.Epics[epicIndex-1]
	sprint := e.data.Sprints[sprintIndex-1]

	// map all custom fields
	unknowns := tcontainer.NewMarshalMap()
	unknowns[e.config.CustomFields.Epics] = epic.Key
	unknowns[e.config.CustomFields.Sprints] = sprint.ID
	unknowns[e.config.CustomFields.StoryPoints] = storyPoints

	// convert configured components
	components := make([]*jira.Component, len(e.config.Components))
	for i, component := range e.config.Components {
		components[i] = &jira.Component{
			Name: component,
		}
	}

	return &jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: e.config.Project,
			},
			Assignee: e.jira.user,
			Reporter: e.jira.user,
			Type: jira.IssueType{
				Name: e.config.IssueType,
			},
			Summary:     summary,
			Description: description,
			Unknowns:    unknowns,
			Components:  components,
			Labels:      e.config.Labels,
		},
	}, nil
}

func (e Editor) issueTemplate() string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 1, 1, 1, ' ', 0)

	// Epics template
	fmt.Fprint(w, "# Epics\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# ID\t|\tKey\t|\tPriority\t|\tSummary\n")
	fmt.Fprint(w, "# --\t|\t---\t|\t--------\t|\t-------\n")
	for i, epic := range e.data.Epics {
		fmt.Fprintf(w, "# %d\t|\t%s\t|\t%s\t|\t%s\n", i+1, epic.Key, epic.Priority, epic.Summary)
	}

	fmt.Fprint(w, "#\n#\n")

	// Sprints template
	fmt.Fprint(w, "# Sprints\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# ID\t|\tName\n")
	fmt.Fprint(w, "# --\t|\t----\n")
	for i, sprint := range e.data.Sprints {
		fmt.Fprintf(w, "# %d\t|\t%s\n", i+1, sprint.Name)
	}

	fmt.Fprint(w, "#\n")

	// Issues template
	fmt.Fprint(w, "# New Issues\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# Epic, Sprint, Summary, Story Points, Description\n")
	fmt.Fprint(w, "\n")

	w.Flush()
	return b.String()
}
