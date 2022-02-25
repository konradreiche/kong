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
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/trivago/tgo/tcontainer"
)

var (
	errMissingColumn     = errors.New("missing column")
	errEpicMismatch      = errors.New("epic does not exist")
	errSprintMismatch    = errors.New("sprint does not exist")
	errUnknownIssue      = errors.New("issues does not exist")
	errUnknownTransition = errors.New("transition does not exist")
)

// Editor provides any functionality that processes user input by providing a
// editor which creates files and parses back the content the user provided.
type Editor struct {
	jira   Jira
	data   Data
	config Config
}

// jewEditor returns a new instace of Editor.
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

// OpenIssueEditor creates a new file create Jira issues in batches.
func (e Editor) OpenIssueEditor(ctx context.Context) error {
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

// CloneEditorArgs defines the arguments being passed to OpenCloneEditor.
type CloneEditorArgs struct {
	Project  string
	Sprint   int
	SPFactor float64
	Issues   Issues
}

// OpenCloneEditor creates a new file create Jira issues in batches.
func (e Editor) OpenCloneEditor(ctx context.Context, args CloneEditorArgs) error {
	filename, cleanup, err := e.createFile(e.cloneTemplate(args), "kong-clone")
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

		keys := make([]string, 0, len(columns))
		for _, column := range columns {
			if column[0] != "c" && column[0] != "clone" {
				continue
			}
			keys = append(keys, column[1])
		}

		// abort if user selected no issues to clone
		if len(keys) == 0 {
			return nil
		}

		return e.jira.CloneIssues(ctx, keys, args.Project, args.Sprint, args.SPFactor)
	}
}

// OpenSprintEditor creates a new file to edit the sprint board issue progress.
func (e Editor) OpenSprintEditor(ctx context.Context) error {
	filename, cleanup, err := e.createFile(e.sprintTemplate(), "kong-sprint")
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

		var updateIssues []issueTransition
		for _, row := range columns {
			action := row[0]
			key := row[1]

			issue, ok := e.data.IssueByKey[key]
			if !ok {
				return errUnknownIssue
			}

			// skip issues without transition to apply
			if action == issue.Status.Acronym {
				continue
			}

			// look up transition based on action specified as acronym
			transition, ok := issue.TransitionsByAcronym[action]
			if !ok {
				return errUnknownTransition
			}

			// construct tuple to perform issue transitions
			updateIssues = append(updateIssues, issueTransition{
				issueKey:   key,
				transition: transition,
			})
		}
		return e.jira.TransitionIssues(ctx, updateIssues)
	}
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
	if epicIndex < 0 || epicIndex > len(e.data.Epics) {
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
				Name: e.config.IssueType,
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

func (e Editor) cloneTemplate(args CloneEditorArgs) string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 1, 1, 1, ' ', 0)

	// use target project's issues to filter out already cloned issues
	existingIssues := make(map[string]Issue, len(args.Issues))
	for _, issue := range args.Issues {
		existingIssues[issue.Summary] = issue
	}
	clonedIssues := make(Issues, 0)

	// List actions and issues
	for _, issue := range e.data.Issues {
		if _, ok := existingIssues[issue.Summary]; ok {
			clonedIssues = append(clonedIssues, issue)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", "keep", issue.Key, issue.Summary)
	}

	fmt.Fprint(w, "\n")

	fmt.Fprintf(
		w,
		"# Cloning issues from project %s to %s, assigning them to sprint %d\n",
		e.config.Project,
		args.Project,
		args.Sprint,
	)

	fmt.Fprintf(w, "# and multiplying the story points by %v\n", args.SPFactor)
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# Commands:\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# c, clone <key> = clone issue to new project\n")
	fmt.Fprint(w, "# k, keep <key> = keep as is, do nothing\n")
	fmt.Fprint(w, "#\n")
	fmt.Fprint(w, "# Cloned:\n")
	fmt.Fprint(w, "#\n")

	for _, issue := range clonedIssues {
		fmt.Fprintf(w, "# %s\t-\t%s\n", issue.Key, issue.Summary)
	}

	w.Flush()
	return b.String()
}

func (e Editor) sprintTemplate() string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 1, 1, 1, ' ', 0)

	// List issues and their status
	for _, issue := range e.data.SprintIssues {
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
