package kong

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/trivago/tgo/tcontainer"
	"golang.org/x/sync/errgroup"
)

// Jira encapsualtes interaction with the Jira API. It exposes a subset of the
// possible interactions in order to simplify the workflow tailored to the
// user.
type Jira struct {
	client *jira.Client
	user   *jira.User
	config Config
}

// NewJira returns a Jira client based on the given username and password.
func NewJira() (Jira, error) {
	config, err := LoadConfig()
	if err != nil {
		return Jira{}, err
	}
	tp := jira.BasicAuthTransport{
		Username: config.Username,
		Password: config.Password,
	}
	client, err := jira.NewClient(tp.Client(), config.Endpoint)
	if err != nil {
		return Jira{}, err
	}
	user, _, err := client.User.GetSelf()
	if err != nil {
		return Jira{}, err
	}
	return Jira{
		client: client,
		user:   user,
		config: config,
	}, nil
}

// ListIssues fetches all issues according to a specific JQL query.
func (j Jira) ListIssues() (Issues, error) {
	conditions := []string{
		"project = " + j.config.Project,
		"issueType IN (Task, Story, Bug)",
		"assignee = \"" + j.user.DisplayName + "\"",
		"status != Closed",
	}
	jql := strings.Join(conditions, " AND ")
	list, _, err := j.client.Issue.Search(jql, &jira.SearchOptions{
		Expand: "transitions",
	})
	if err != nil {
		return nil, err
	}
	return NewIssues(list), nil
}

// ListSprintIssues fetches all issues assigned to the current sprint.
func (j Jira) ListSprintIssues() (Issues, error) {
	conditions := []string{
		"project = " + j.config.Project,
		"issueType IN (Story, Bug)",
		"assignee = \"" + j.user.DisplayName + "\"",
		"status != Closed",
		"sprint in openSprints()",
	}
	jql := strings.Join(conditions, " AND ")
	list, _, err := j.client.Issue.Search(jql, &jira.SearchOptions{
		Expand: "transitions",
	})
	if err != nil {
		return nil, err
	}
	return NewIssues(list), nil
}

// ListEpics returns a list of epics associated wtih the current project.
func (j Jira) ListEpics() (Issues, error) {
	conditions := []string{
		"project = " + j.config.Project,
		"issueType = Epic",
		"status != Closed",
	}

	// include query for labels if configured
	if len(j.config.Labels) > 0 {
		label := "labels IN (" + strings.Join(j.config.Labels, ",") + ")"
		conditions = append(conditions, label)

	}

	jql := strings.Join(conditions, " AND ")
	issues, _, err := j.client.Issue.Search(jql, &jira.SearchOptions{})
	if err != nil {
		return nil, err
	}
	return NewIssues(issues), nil
}

// ListSprints fetches all active and future sprints for the configured board
// and the specified keyword.
func (j Jira) ListSprints() (Sprints, error) {
	sprints, _, err := j.client.Board.GetAllSprintsWithOptions(
		j.config.BoardID,
		&jira.GetAllSprintsOptions{
			State: "active,future",
		},
	)
	if err != nil {
		return nil, err
	}

	// only return sprints that contain the configured keyword
	filtered := make([]jira.Sprint, 0, len(sprints.Values))
	for _, s := range sprints.Values {
		if strings.Contains(s.Name, j.config.SprintKeyword) {
			filtered = append(filtered, s)
		}
	}
	return NewSprints(filtered), nil
}

// GetBoardID returns the board ID for a given project.
func (j Jira) GetBoardID(project string) (int, error) {
	req, err := j.client.NewRequest("GET", "/rest/agile/1.0/board?projectKeyOrId="+project, nil)
	if err != nil {
		return 0, err
	}

	resp, err := j.client.Do(req, nil)
	if err != nil {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		return 0, errors.New(string(b))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Values []struct {
			ID int `json:"id"`
		} `json:"values"`
	}

	if err := json.Unmarshal(b, &result); err != nil {
		return 0, err
	}

	return result.Values[0].ID, nil
}

// ListSprintsForBoard returns a list of sprints for the given board ID.
func (j Jira) ListSprintsForBoard(boardID int) (Sprints, error) {
	sprints, _, err := j.client.Board.GetAllSprintsWithOptions(
		boardID,
		&jira.GetAllSprintsOptions{
			State: "active,future",
		},
	)
	if err != nil {
		return nil, err
	}
	return NewSprints(sprints.Values), nil
}

// CreateIssues creates the given issues in parallel.
func (j Jira) CreateIssues(ctx context.Context, issues []*jira.Issue) error {
	g, _ := errgroup.WithContext(ctx)

	for _, issue := range issues {
		// allocate variable to avoid scope capturing
		issue := issue

		// create issues concurrency
		g.Go(func() error {
			newIssue, resp, err := j.client.Issue.Create(issue)
			if err != nil {
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				return errors.New(string(b))
			}
			fmt.Printf("Created %s - %s\n", newIssue.Key, issue.Fields.Summary)
			return nil
		})
	}
	return g.Wait()
}

// CreateSprint creates a new sprint.
func (j Jira) CreateSprint(name string, month, day int) error {
	// configure start and end date
	now := time.Now()
	tz := now.Location()
	layout := "2006-01-02T15:04:05.000-07:00"
	startDate := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, tz)
	startDate.Format(layout)

	// define end date based on configured sprint duration
	endDate := startDate.Add(time.Duration(j.config.SprintDuration+1) * 24 * time.Hour)

	// define payload
	payload := struct {
		Name          string `json:"name"`
		StartDate     string `json:"startDate"`
		EndDate       string `json:"endDate"`
		OriginBoardID int    `json:"originBoardId"`
		Goal          string `json:"goal,omitempty"`
	}{
		Name:          fmt.Sprintf("%s %d/%d", name, month, day),
		StartDate:     startDate.Format(layout),
		EndDate:       endDate.Format(layout),
		OriginBoardID: j.config.BoardID,
	}

	req, err := j.client.NewRequest("POST", "/rest/agile/1.0/sprint", payload)
	if err != nil {
		return err
	}

	resp, err := j.client.Do(req, nil)
	if err != nil {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return ErrCreateSprint(string(b))
	}

	return nil
}

// CloneIssues clones a list of issues identified by their keys to a new project.
func (j Jira) CloneIssues(ctx context.Context, keys []string, project string, sprint int, spFactor float64) error {
	conditions := []string{
		"project = " + j.config.Project,
		"key IN (" + strings.Join(keys, ", ") + ")",
	}
	jql := strings.Join(conditions, " AND ")
	list, _, err := j.client.Issue.Search(jql, &jira.SearchOptions{})
	if err != nil {
		return err
	}

	issues := make([]*jira.Issue, len(list))
	for i, issue := range list {
		unknowns := tcontainer.NewMarshalMap()
		unknowns[j.config.CustomFields.Sprints] = sprint

		// adjust story points
		storyPoints, ok := issue.Fields.Unknowns[j.config.CustomFields.StoryPoints].(float64)
		if !ok {
			return fmt.Errorf(
				"unexpected type for story points field: %T",
				issue.Fields.Unknowns[j.config.CustomFields.StoryPoints])
		}
		unknowns[j.config.CustomFields.StoryPoints] = math.Ceil(storyPoints * spFactor)

		issues[i] = &jira.Issue{
			Fields: &jira.IssueFields{
				// set project to clone target
				Project: jira.Project{
					Key: project,
				},
				Assignee:    issue.Fields.Assignee,
				Reporter:    issue.Fields.Reporter,
				Type:        issue.Fields.Type,
				Summary:     issue.Fields.Summary,
				Description: issue.Fields.Description,
				Unknowns:    unknowns,
			},
		}
	}

	return j.CreateIssues(ctx, issues)
}
