package kong

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/google/go-cmp/cmp"
)

func TestListIssues(t *testing.T) {
	const (
		maxResults = 2
		total      = 5
	)
	var gotCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifyMaxResultsResponse(t, r, maxResults)
		writeSearchResponse(t, w, r, maxResults, total, gotCalled)
		gotCalled++
	}))

	jira := newJiraTest(t, ts, maxResults)
	got, err := jira.ListIssues(context.Background(), "KONG")
	if err != nil {
		t.Fatal(err)
	}
	wantCalled := 3
	if gotCalled != wantCalled {
		t.Errorf("got %d, want: %d", gotCalled, wantCalled)
	}

	want := Issues{
		issue("KONG-1"),
		issue("KONG-2"),
		issue("KONG-3"),
		issue("KONG-4"),
		issue("KONG-5"),
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("diff: %s", diff)
	}
}

func TestSearch(t *testing.T) {
	const (
		total      = 5
		maxResults = 2
	)

	var gotCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSearchResponse(t, w, r, maxResults, total, gotCalled)
		gotCalled++
	}))

	jira := newJiraTest(t, ts, maxResults)
	got, err := jira.search(context.Background(), "project = KONG")
	if err != nil {
		t.Fatal(err)
	}
	wantCalled := 3
	if gotCalled != wantCalled {
		t.Errorf("got %d, want: %d", gotCalled, wantCalled)
	}

	want := Issues{
		issue("KONG-1"),
		issue("KONG-2"),
		issue("KONG-3"),
		issue("KONG-4"),
		issue("KONG-5"),
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("diff: %s", diff)
	}
}

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
