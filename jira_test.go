package kong

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/google/go-cmp/cmp"
)

func TestListIssues(t *testing.T) {
	var gotCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const (
			maxResults = 2
			total      = 5
		)
		writeSearchResponse(t, w, maxResults, total, gotCalled)
		gotCalled++
	}))

	jira := newJiraTest(t, ts)
	got, err := jira.ListIssues(context.Background(), "KONG")
	if err != nil {
		t.Fatal(err)
	}
	wantCalled := 2
	if gotCalled != wantCalled {
		t.Errorf("got %d, want: %d", gotCalled, wantCalled)
	}

	want := Issues{
		issue("KONG-1"),
		issue("KONG-2"),
		issue("KONG-3"),
		issue("KONG-4"),
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
		Priority: "Major",
		Status: Status{
			Name:    "Done",
			Acronym: "d",
		},
		Transitions:             []Transition{},
		TransitionsByAcronym:    map[string]Transition{},
		OrderByTransitionStatus: map[string]int{},
	}
}

func newJiraTest(t *testing.T, ts *httptest.Server) *Jira {
	client, err := jira.NewClient(ts.Client(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &Jira{
		client: client,
		user: &jira.User{
			DisplayName: "King",
		},
	}
}

func writeSearchResponse(t *testing.T, w http.ResponseWriter, maxResults, total, called int) {
	data := searchResult{
		Issues:     make([]jira.Issue, maxResults),
		StartAt:    0,
		MaxResults: maxResults,
		Total:      total,
	}

	for i := 0; i < maxResults; i++ {
		id := called*maxResults + i + 1
		data.Issues[i] = jira.Issue{
			ID:     fmt.Sprint(id),
			Key:    fmt.Sprintf("KONG-%d", id),
			Expand: "transition",
			Fields: &jira.IssueFields{
				Priority: &jira.Priority{
					Name: "Major",
				},
				Status: &jira.Status{
					Name: "Done",
				},
			},
		}
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(data); err != nil {
		t.Fatal(err)
	}
}
