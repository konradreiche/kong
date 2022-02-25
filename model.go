package kong

import (
	"strings"
	"time"
	"unicode"

	"github.com/andygrunwald/go-jira"
)

// Issues is a list of issues which conveniently exposes a Print method to
// display the issue's data.
type Issues []Issue

// Sprints is a list of sprints which conveniently exposes a Print method to
// display sprints.
type Sprints []Sprint

// Issue is a Jira issue abstraction. The type primarily exists to only
// serialize a subset of the data to disk.
type Issue struct {
	Key                  string
	Summary              string
	Priority             string
	Status               Status
	Transitions          []Transition
	TransitionsByAcronym map[string]Transition
}

// Transition is a Jira transition abstraction. The type primarily exists to
// only serialize a subset of data to disk.
type Transition struct {
	ID          string
	Name        string
	Description string
	Acronym     string
}

// Status is a Jira status abstraction.
type Status struct {
	Name    string
	Acronym string
}

// Sprint is a Jira sprint abstraction.  The type primarily exists to only
// serialize a subset of the data to disk.
type Sprint struct {
	ID      int
	Name    string
	State   string
	EndDate time.Time
}

// NewIssues returns a new instance of Issues by converting jira.Issue to
// Issue.
func NewIssues(issues []jira.Issue) Issues {
	result := make(Issues, len(issues))
	transitions := make([]Transition, 0)
	transitionsByAcronym := make(map[string]Transition)

	for i, issue := range issues {
		result[i] = Issue{
			Key:      issue.Key,
			Summary:  issue.Fields.Summary,
			Priority: issue.Fields.Priority.Name,
			Status: Status{
				Name:    issue.Fields.Status.Name,
				Acronym: statusAcronym(issue.Fields.Status.Name),
			},
		}

		// only initialize list of transitions once
		if len(transitions) == 0 {
			transitions = make([]Transition, len(issue.Transitions))
			for j, transition := range issue.Transitions {
				acronym := statusAcronym(transition.To.Name)
				t := Transition{
					ID:          transition.ID,
					Name:        transition.To.Name,
					Description: transition.To.Description,
					Acronym:     acronym,
				}
				transitions[j] = t
				transitionsByAcronym[acronym] = t
			}
		}

		// then reference the initialized lists
		result[i].Transitions = transitions
		result[i].TransitionsByAcronym = transitionsByAcronym
	}
	return result
}

// NewSprints returns a new instance of Sprints by converting jira.Sprint to
// Sprint.
func NewSprints(sprints []jira.Sprint) Sprints {
	result := make(Sprints, len(sprints))
	for i, sprint := range sprints {
		result[i] = NewSprint(sprint)
	}
	return result
}

// NewSprint returns a new instance of Sprint by converting jira.Sprint to
// Sprint.
func NewSprint(sprint jira.Sprint) Sprint {
	s := Sprint{
		ID:    sprint.ID,
		Name:  sprint.Name,
		State: sprint.State,
	}
	if sprint.EndDate != nil {
		s.EndDate = *sprint.EndDate
	}
	return s
}

// ActiveSprint returns the currently active sprint or an error if there is no
// active sprint.
func (s Sprints) ActiveSprint() (Sprint, error) {
	for _, sprint := range s {
		if sprint.State == "active" {
			return sprint, nil
		}
	}
	return Sprint{}, ErrNoActiveSprint
}

// Transitions returns a list of transitions from one of the issues since each
// issue should have the same set of transitions.
func (i Issues) Transitions() []Transition {
	if len(i) == 0 {
		return nil
	}
	return i[0].Transitions
}

func statusAcronym(name string) string {
	var s strings.Builder
	for _, word := range strings.Split(name, " ") {
		s.WriteRune(unicode.ToLower(rune(word[0])))
	}
	return s.String()
}
