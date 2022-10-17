package kong

import (
	"errors"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/google/go-cmp/cmp"
)

func TestNewIssues(t *testing.T) {
	tests := []struct {
		name         string
		issues       []jira.Issue
		customFields CustomFields
		want         Issues
		wantError    error
	}{
		{
			name:         "nil",
			issues:       []jira.Issue{},
			customFields: CustomFields{},
			want:         Issues{},
		},
		{
			name: "fields-nil",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
				},
			},
			wantError: errJiraFieldEmpty,
		},
		{
			name: "summary-empty",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
					Fields: &jira.IssueFields{
						Priority: &jira.Priority{
							Name: "Major",
						},
					},
				},
			},
			wantError: errJiraSummaryEmpty,
		},
		{
			name: "priority-nil",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
					Fields: &jira.IssueFields{
						Summary: "Add command to list issues",
					},
				},
			},
			wantError: errJiraPriorityNil,
		},
		{
			name: "priority-name-empty",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
					Fields: &jira.IssueFields{
						Summary:  "Add command to list issues",
						Priority: &jira.Priority{},
					},
				},
			},
			wantError: errJiraPriorityNameEmpty,
		},
		{
			name: "transitions-empty",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
					Fields: &jira.IssueFields{
						Summary: "Add command to list issues",
						Priority: &jira.Priority{
							Name: "Major",
						},
					},
				},
			},
			wantError: errJiraTransitionsEmpty,
		},
		{
			name: "valid",
			issues: []jira.Issue{
				{
					Key: "KONG-1",
					Fields: &jira.IssueFields{
						Summary: "Add command to list issues",
						Priority: &jira.Priority{
							Name: "Major",
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
				},
			},
			want: Issues{
				{
					Key:      "KONG-1",
					Summary:  "Add command to list issues",
					Priority: "Major",
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
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewIssues(tt.issues, tt.customFields)
			if !errors.Is(err, tt.wantError) {
				t.Errorf("got %v, want: %v", err, tt.wantError)
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}

func TestNewSprint(t *testing.T) {
	t.Run("without-end-date", func(t *testing.T) {
		sprint := jira.Sprint{
			ID:    1,
			Name:  "Komodo",
			State: "active",
		}
		got := NewSprint(sprint)
		want := Sprint{
			ID:    1,
			Name:  "Komodo",
			State: "active",
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("diff: %s", diff)
		}
	})

	t.Run("with-end-date", func(t *testing.T) {
		sprint := jira.Sprint{
			ID:    1,
			Name:  "Komodo",
			State: "active",
		}
		endDate := time.Date(1933, time.April, 7, 0, 0, 0, 0, time.UTC)
		sprint.EndDate = &endDate

		got := NewSprint(sprint)
		want := Sprint{
			ID:      1,
			Name:    "Komodo",
			State:   "active",
			EndDate: endDate,
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("diff: %s", diff)
		}
	})
}

func transition(id, name, description, acronym string) Transition {
	return Transition{
		ID:          id,
		Name:        name,
		Description: description,
		Acronym:     acronym,
	}
}
