package kong

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/andygrunwald/go-jira"
)

type searchResult struct {
	Issues     []jira.Issue `json:"issues"`
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
}

func issue(key string) Issue {
	return Issue{
		Key:      key,
		Summary:  "The epic clash between two titans",
		Priority: "Major",
		Status: Status{
			Name:    "Done",
			Acronym: "d",
		},
		Transitions: []Transition{
			transition("1", "To Do", "Ticket has yet to be started.", "td"),
			transition("2", "In Progress", "This issue is currently being worked on.", "ip"),
		},
		TransitionsByAcronym: map[string]Transition{
			"td": transition("1", "To Do", "Ticket has yet to be started.", "td"),
			"ip": transition("2", "In Progress", "This issue is currently being worked on.", "ip"),
		},
		OrderByTransitionStatus: map[string]int{
			"To Do":       0,
			"In Progress": 1,
		},
	}
}

func newJiraTest(t *testing.T, ts *httptest.Server, maxResults int) *Jira {
	client, err := jira.NewClient(ts.Client(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &Jira{
		client: client,
		user: &jira.User{
			DisplayName: "King",
		},
		maxResults: maxResults,
	}
}

func verifyMaxResultsResponse(t *testing.T, r *http.Request, want int) {
	t.Helper()

	value := r.URL.Query().Get("maxResults")
	maxResults, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	got := int(maxResults)
	if got != want {
		t.Errorf("got %d, want: %d", got, want)
	}
}

func writeSearchResponse(t *testing.T, w http.ResponseWriter, r *http.Request, maxResults, total, called int) {
	t.Helper()

	start := maxResults * called
	data := searchResult{
		Issues:     make([]jira.Issue, 0),
		StartAt:    start,
		MaxResults: maxResults,
		Total:      total,
	}
	end := maxResults * (called + 1)
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		id := i + 1
		issue := jira.Issue{
			ID:     fmt.Sprint(id),
			Key:    fmt.Sprintf("KONG-%d", id),
			Expand: "transition",
			Fields: &jira.IssueFields{
				Summary: "The epic clash between two titans",
				Priority: &jira.Priority{
					Name: "Major",
				},
				Status: &jira.Status{
					Name: "Done",
				},
			},
			Transitions: []jira.Transition{
				{
					ID: "1",
					To: jira.Status{
						Name:        "To Do",
						Description: "Ticket has yet to be started.",
					},
				},
				{
					ID: "2",
					To: jira.Status{
						Name:        "In Progress",
						Description: "This issue is currently being worked on.",
					},
				},
			},
		}
		data.Issues = append(data.Issues, issue)
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(data); err != nil {
		t.Fatal(err)
	}
}
