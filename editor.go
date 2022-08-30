package kong

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/trivago/tgo/tcontainer"
	"gopkg.in/yaml.v2"
)

var (
	errMissingColumn     = errors.New("missing column")
	errParentMismatch    = errors.New("epic or initiative does not exist")
	errSprintMismatch    = errors.New("sprint does not exist")
	errUnknownIssue      = errors.New("issue does not exist")
	errUnknownTransition = errors.New("transition does not exist")
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

func (e Editor) createFile(template, filename string) (string, func(), error) {
	f, err := os.CreateTemp(os.TempDir(), filename)
	if err != nil {
		return "", nil, err
	}
	_, err = f.WriteString(template)
	if err != nil {
		return "", nil, err
	}
	err = f.Close()
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		if err := os.Remove(f.Name()); err != nil {
			fmt.Fprint(os.Stderr, err)
		}
	}

	return f.Name(), cleanup, nil
}

// OpenNewIssueEditor creates a new file create Jira issues in batches.
func (e Editor) OpenNewIssueEditor(ctx context.Context) error {
	filename, cleanup, err := e.createFile(e.issueTemplate(), "kong-new-issues")
	if err != nil {
		return err
	}
	defer cleanup()

	for {
		if err := e.open(ctx, filename, true); err != nil {
			return err
		}
		b, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		lines := e.parseLines(string(b))

		// abort on empty input
		if len(lines) == 0 {
			return nil
		}

		columns, err := e.parseColumns(lines, 5)
		if err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}

		issues, err := e.parseIssues(columns, e.config.IssueType)
		if err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}
		return e.jira.CreateIssues(ctx, issues)
	}
}

func (e Editor) OpenEditIssueEditor(ctx context.Context, key string) error {
	issue, ok := e.data.IssueByKey[key]
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownIssue, key)
	}
	b, err := yaml.Marshal(issue)
	if err != nil {
		return err
	}
	filename, cleanup, err := e.createFile(e.editIssueTemplate(key, b), "kong-edit-issue")
	if err != nil {
		return err
	}
	defer cleanup()

	for {
		if err := e.open(ctx, filename, true); err != nil {
			return err
		}
		b, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		var issue Issue
		if err := yaml.Unmarshal(b, &issue); err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}
		return e.jira.UpdateIssue(ctx, key, issue)
	}
}

// OpenEpicEditor creates a new file create Jira epics in batches.
func (e Editor) OpenEpicEditor(ctx context.Context) error {
	filename, cleanup, err := e.createFile(e.epicTemplate(), "kong-new-epics")
	if err != nil {
		return err
	}
	defer cleanup()

	for {
		if err := e.open(ctx, filename, true); err != nil {
			return err
		}
		b, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		lines := e.parseLines(string(b))

		// abort on empty input
		if len(lines) == 0 {
			return nil
		}

		columns, err := e.parseColumns(lines, 5)
		if err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}

		epics, err := e.parseIssues(columns, "Epic")
		if err != nil {
			fmt.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}
		return e.jira.CreateIssues(ctx, epics)
	}
}

// OpenSprintEditor creates a new file to edit the sprint board issue progress.
func (e Editor) OpenSprintEditor(ctx context.Context, includeDone bool) error {
	filename, cleanup, err := e.createFile(e.sprintTemplate(includeDone), "kong-sprint")
	if err != nil {
		return err
	}
	defer cleanup()

	for {
		if err := e.open(ctx, filename, false); err != nil {
			return err
		}
		b, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		lines := e.parseLines(string(b))

		// abort on empty input
		if len(lines) == 0 {
			return nil
		}

		columns, err := e.parseActionColumns(lines)
		if err != nil {
			return err
		}

		var (
			issueTransitions    []issueTransition
			moveIssuesToBacklog []string
		)

		for _, row := range columns {
			action := row[0]
			key := row[1]

			issue, ok := e.data.IssueByKey[key]
			if !ok {
				return fmt.Errorf("%w: %s", errUnknownIssue, key)
			}

			// skip issues without transition to apply
			if action == issue.Status.Acronym {
				continue
			}

			if action == backlogAcronym {
				moveIssuesToBacklog = append(moveIssuesToBacklog, key)
				continue
			}

			// look up transition based on action specified as acronym
			transition, ok := issue.TransitionsByAcronym[action]
			if !ok {
				return errUnknownTransition
			}

			// construct tuple to perform issue transitions
			issueTransitions = append(issueTransitions, issueTransition{
				issueKey:   key,
				transition: transition,
			})
		}
		if err := e.jira.MoveIssuesToBacklog(ctx, moveIssuesToBacklog); err != nil {
			return err
		}
		return e.jira.TransitionIssues(ctx, issueTransitions)
	}
}

// OpenStandupEditor creates a new file to edit the sprint board issue progress.
func (e Editor) OpenStandupEditor(ctx context.Context, standupType string) error {
	var buf bytes.Buffer

	switch standupType {
	case "sprint":
		text := e.config.SprintStandupTemplate
		tmpl, err := template.New("standup").Parse(text)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(&buf, e.data.SprintIssues); err != nil {
			return err
		}
	case "epics":
		text := e.config.EpicStandupTemplate
		tmpl, err := template.New("standup").Parse(text)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(&buf, e.data.Epics); err != nil {
			return err
		}
	}

	filename, cleanup, err := e.createFile(buf.String(), "kong-standup")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := e.open(ctx, filename, false); err != nil {
		return err
	}

	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	cmd := exec.Command(e.config.CopyCommand)
	cmd.Stdin = bytes.NewBuffer(b)
	return cmd.Run()
}

func (e Editor) open(ctx context.Context, filename string, lastLine bool) error {
	args := []string{filename}
	if lastLine {
		args = append(args, "-c", "norm! G")
	}

	cmd := exec.CommandContext(ctx, "vim", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

func (e Editor) parseColumns(lines []string, numColumns int) ([][]string, error) {
	columns := make([][]string, len(lines))
	for i, line := range lines {
		columns[i] = strings.SplitN(line, ",", numColumns)
		if len(columns[i]) != numColumns {
			return nil, errMissingColumn
		}
	}
	return columns, nil
}

func (e Editor) parseActionColumns(lines []string) ([][]string, error) {
	columns := make([][]string, len(lines))
	for i, line := range lines {
		columns[i] = strings.Fields(line)
		if len(columns[i]) < 3 {
			return nil, errMissingColumn
		}
	}
	return columns, nil
}

func (e Editor) parseIssues(columns [][]string, issueType string) ([]*jira.Issue, error) {
	issues := make([]*jira.Issue, 0)
	for _, c := range columns {
		issue, err := e.parseIssue(c, issueType)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (e Editor) parseIssue(columns []string, issueType string) (*jira.Issue, error) {
	parentIndex, err := strconv.Atoi(columns[0])
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

	// handle issue and epic creations differently
	parents := e.data.Epics
	if issueType == "Epic" {
		parents = e.data.Initiatives
	}

	// verify parent index matches available parents
	if parentIndex < 0 || parentIndex > len(parents) {
		return nil, errParentMismatch
	}

	// verify sprint index matches available sprints
	if sprintIndex < 0 || sprintIndex > len(e.data.Sprints) {
		return nil, errSprintMismatch
	}

	// map all custom fields
	unknowns := tcontainer.NewMarshalMap()
	unknowns[e.config.CustomFields.StoryPoints] = storyPoints

	// setting epic or sprint to 0 means unassigned
	if parentIndex != 0 && issueType == e.config.IssueType {
		epic := e.data.Epics[parentIndex-1]
		unknowns[e.config.CustomFields.Epics] = epic.Key
	}

	// issues and epics have both different custom fields to set
	if parentIndex != 0 && issueType == "Epic" {
		initiative := e.data.Initiatives[parentIndex-1]
		unknowns[e.config.CustomFields.EpicName] = summary
		unknowns[e.config.CustomFields.ParentLink] = initiative.Key
	}

	var dueDate time.Time
	if sprintIndex != 0 {
		sprint := e.data.Sprints[sprintIndex-1]
		unknowns[e.config.CustomFields.Sprints] = sprint.ID

		// set issue due date to end of sprint if defined
		if !sprint.EndDate.IsZero() {
			dueDate = sprint.EndDate
		}
	}

	// convert configured components
	components := make([]*jira.Component, len(e.config.Components))
	for i, component := range e.config.Components {
		components[i] = &jira.Component{
			Name: component,
		}
	}

	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: e.config.Project,
			},
			Assignee: e.jira.user,
			Reporter: e.jira.user,
			Type: jira.IssueType{
				Name: issueType,
			},
			Summary:     summary,
			Description: description,
			Unknowns:    unknowns,
			Components:  components,
			Labels:      e.config.Labels,
		},
	}

	if !dueDate.IsZero() {
		issue.Fields.Duedate = jira.Date(dueDate)
	}

	return &issue, nil
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

func (e Editor) epicTemplate() string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 1, 1, 1, ' ', 0)

	// determine number of dashes for key and summary column
	keyBorder := strings.Repeat("-", e.maxInitiativeKeyLength())
	summaryBorder := strings.Repeat("-", e.maxInitiativeSummaryLength())

	// Epics template
	fmt.Fprint(w, "# Initiatives\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# ID\t|\tKey\t|\tPriority\t|\tSummary\n")
	fmt.Fprintf(w, "# --\t|\t%s\t|\t--------\t|\t%s\n", keyBorder, summaryBorder)
	fmt.Fprintf(w, "# %d\t|\t%s\t|\t%s\t|\t%s\n", 0, "", "", "Unassigned")
	fmt.Fprintf(w, "# --\t|\t%s\t|\t--------\t|\t%s\n", keyBorder, summaryBorder)

	for i, initiative := range e.data.Initiatives {
		fmt.Fprintf(w, "# %d\t|\t%s\t|\t%s\t|\t%s\n", i+1, initiative.Key, initiative.Priority, initiative.Summary)
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

	// Epics template
	fmt.Fprint(w, "# New Epics\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# Initiative, Sprint, Summary, Story Points, Description\n")
	fmt.Fprint(w, "\n")

	w.Flush()
	return b.String()
}

func (e Editor) sprintTemplate(includeDone bool) string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 1, 1, 1, ' ', 0)

	// List issues and their status
	for _, issue := range e.data.SprintIssues.Sort() {
		if issue.Status.IsDone && !includeDone {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", issue.Status.Acronym, issue.Key, issue.Summary)
	}
	fmt.Fprint(w, "\n")

	fmt.Fprintf(w, "# Update the status of any sprint issues\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# Commands:\n")
	fmt.Fprint(w, "#\n")

	for _, t := range e.data.SprintIssues.Transitions() {
		fmt.Fprintf(w, "# %s\t<key> =\t%s\n", t.Acronym, t.Name)
	}
	fmt.Fprint(w, "#\n")
	fmt.Fprintf(w, "# %s\t<key> =\tMove into backlog\n", backlogAcronym)

	w.Flush()
	return b.String()
}

func (e Editor) editIssueTemplate(key string, yaml []byte) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# %s\n", key)
	fmt.Fprint(&b, string(yaml))
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

func (e Editor) maxInitiativeKeyLength() int {
	var max int
	for _, initiative := range e.data.Initiatives {
		if len(initiative.Key) > max {
			max = len(initiative.Key)
		}
	}
	return max
}

func (e Editor) maxInitiativeSummaryLength() int {
	var max int
	for _, initiative := range e.data.Initiatives {
		if len(initiative.Summary) > max {
			max = len(initiative.Summary)
		}
	}
	return max
}
