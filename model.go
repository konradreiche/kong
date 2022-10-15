package kong

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/andygrunwald/go-jira"
)

var (
	errJiraKeyEmpty          = errors.New("key cannot be empty")
	errJiraFieldEmpty        = errors.New("fields cannot be nil")
	errJiraSummaryEmpty      = errors.New("summary cannot be empty")
	errJiraPriorityNil       = errors.New("priority cannot be nil")
	errJiraPriorityNameEmpty = errors.New("priority name cannot be empty")
	errJiraTransitionsEmpty  = errors.New("transitions cannot be empty")
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
	Key                     string                `yaml:"-"`
	Summary                 string                `yaml:"summary"`
	Priority                string                `yaml:"-"`
	Status                  Status                `yaml:"-"`
	Transitions             []Transition          `yaml:"-"`
	TransitionsByAcronym    map[string]Transition `yaml:"-"`
	OrderByTransitionStatus map[string]int        `yaml:"-"`
	SprintID                int                   `yaml:"sprintID"`
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
	IsDone  bool
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
func NewIssues(jiraIssues []jira.Issue, customFields CustomFields) (Issues, error) {
	result := make(Issues, len(jiraIssues))
	transitions := make([]Transition, 0)
	transitionsByAcronym := make(map[string]Transition)
	orderByTransitionStatus := make(map[string]int, len(transitions))

	for i, jiraIssue := range jiraIssues {
		issue, err := NewIssue(jiraIssue)
		if err != nil {
			return nil, fmt.Errorf("NewIssues: %w", err)
		}
		result[i] = issue

		// only initialize list of transitions once
		if len(transitions) == 0 {
			transitions = make([]Transition, len(jiraIssue.Transitions))
			for j, transition := range jiraIssue.Transitions {
				acronym := statusAcronym(transition.To.Name)
				t := Transition{
					ID:          transition.ID,
					Name:        transition.To.Name,
					Description: transition.To.Description,
					Acronym:     acronym,
				}
				transitions[j] = t
				transitionsByAcronym[acronym] = t
				orderByTransitionStatus[transition.To.Name] = j
			}
		}

		// then reference the initialized lists
		result[i].Transitions = transitions
		result[i].TransitionsByAcronym = transitionsByAcronym
		result[i].OrderByTransitionStatus = orderByTransitionStatus

		// set sprint
		if jiraIssue.Fields.Unknowns[customFields.Sprints] != nil {
			sprints := jiraIssue.Fields.Unknowns[customFields.Sprints].([]interface{})
			for _, item := range sprints {
				sprint := item.(map[string]interface{})
				if sprint["state"] == "active" {
					result[i].SprintID = int(sprint["id"].(float64))
					break
				}
			}
		}
	}
	return result, nil
}

// NewIssue returns a new instance of Issue by converting jira.Issue to Issue.
func NewIssue(issue jira.Issue) (Issue, error) {
	if err := validateJiraIssue(issue); err != nil {
		return Issue{}, err
	}
	result := Issue{
		Key:      issue.Key,
		Summary:  issue.Fields.Summary,
		Priority: issue.Fields.Priority.Name,
		Status:   NewStatus(issue),
	}
	return result, nil
}

func validateJiraIssue(issue jira.Issue) error {
	if issue.Key == "" {
		return errJiraKeyEmpty
	}
	if issue.Fields == nil {
		return errJiraFieldEmpty
	}
	if issue.Fields.Summary == "" {
		return errJiraSummaryEmpty
	}
	if issue.Fields.Priority == nil {
		return errJiraPriorityNil
	}
	if issue.Fields.Priority.Name == "" {
		return errJiraPriorityNameEmpty
	}
	if len(issue.Transitions) == 0 {
		return errJiraTransitionsEmpty
	}
	return nil
}

// NewStauts returns a new instnace of Status by converting the status field of
// jira.Issue.
func NewStatus(issue jira.Issue) Status {
	if issue.Fields == nil {
		return Status{}
	}
	if issue.Fields.Status == nil {
		return Status{}
	}
	return Status{
		Name:    issue.Fields.Status.Name,
		Acronym: statusAcronym(issue.Fields.Status.Name),
		IsDone:  issue.Fields.Status.StatusCategory.Key == "done",
	}
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

func (i Issues) Sort() Issues {
	sort.Slice(i, func(a, b int) bool {
		return i[a].OrderByTransitionStatus[i[a].Status.Name] <
			i[b].OrderByTransitionStatus[i[b].Status.Name]
	})
	return i
}

// order by implicit transition status order returned from the Jira API

func statusAcronym(name string) string {
	var s strings.Builder
	for _, word := range strings.Split(name, " ") {
		s.WriteRune(unicode.ToLower(rune(word[0])))
	}
	return s.String()
}
