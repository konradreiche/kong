package kong

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
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
		"issueType IN (Story, Bug)",
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
