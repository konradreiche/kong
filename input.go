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
func NewEditor(ctx context.Context) (Editor, error) {
	var (
		editor Editor
		err    error
	)

	// assign Jira instance
	editor.jira, err = NewJira()
	if err != nil {
		return editor, err
	}
	editor.config = editor.jira.config

	// load data from file or synchronously by calling Jira API directly
	editor.data, err = LoadData()
	if err != nil {
		return editor, err
	}
	if editor.data.Stale() {
		editor.data, err = LoadDataBlocking(ctx)
		if err != nil {
			return editor, err
		}
	}
	return editor, nil
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

		columns, err := e.parseColumns(lines)
		if err != nil {
			return err
		}

		issues, err := e.parseIssues(columns)
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

func (e Editor) parseColumns(lines []string) ([][]string, error) {
	columns := make([][]string, len(lines))
	for i, line := range lines {
		columns[i] = strings.SplitN(line, ",", 5)
		if len(columns[i]) != 5 {
			return nil, errMissingColumn
		}
	}
	return columns, nil
}

func (e Editor) parseIssues(columns [][]string) ([]*jira.Issue, error) {
	issues := make([]*jira.Issue, 0)
	for _, c := range columns {
		issue, err := e.parseIssue(c)
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

	// map all custom fields
	unknowns := tcontainer.NewMarshalMap()
	unknowns[e.config.CustomFields.StoryPoints] = storyPoints

	// setting epic or sprint to 0 means unassigned
	if epicIndex != 0 {
		epic := e.data.Epics[epicIndex-1]
		unknowns[e.config.CustomFields.Epics] = epic.Key
	}

	if sprintIndex != 0 {
		sprint := e.data.Sprints[sprintIndex-1]
		unknowns[e.config.CustomFields.Sprints] = sprint.ID
	}

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

	// determine number of dashes for key and summary column
	keyBorder := strings.Repeat("-", e.maxEpicKeyLength())
	summaryBorder := strings.Repeat("-", e.maxEpicSummaryLength())

	// Epics template
	fmt.Fprint(w, "# Epics\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# ID\t|\tKey\t|\tPriority\t|\tSummary\n")
	fmt.Fprintf(w, "# --\t|\t%s\t|\t--------\t|\t%s\n", keyBorder, summaryBorder)
	fmt.Fprintf(w, "# %d\t|\t%s\t|\t%s\t|\t%s\n", 0, "", "", "Unassigned")
	fmt.Fprintf(w, "# --\t|\t%s\t|\t--------\t|\t%s\n", keyBorder, summaryBorder)

	for i, epic := range e.data.Epics {
		fmt.Fprintf(w, "# %d\t|\t%s\t|\t%s\t|\t%s\n", i+1, epic.Key, epic.Priority, epic.Summary)
	}

	fmt.Fprintf(w, "# --\t|\t%s\t|\t--------\t|\t%s\n", keyBorder, summaryBorder)
	fmt.Fprint(w, "#\n#\n")

	// Sprints template
	fmt.Fprint(w, "# Sprints\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# ID\t|\tName\n")
	fmt.Fprint(w, "# --\t|\t----\n")
	fmt.Fprint(w, "# 0\t|\tUnassigned\n")
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

func (e Editor) maxEpicKeyLength() int {
	var max int
	for _, epic := range e.data.Epics {
		if len(epic.Key) > max {
			max = len(epic.Key)
		}
	}
	return max
}

func (e Editor) maxEpicSummaryLength() int {
	var max int
	for _, epic := range e.data.Epics {
		if len(epic.Summary) > max {
			max = len(epic.Summary)
		}
	}
	return max
}
