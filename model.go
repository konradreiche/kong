package kong

import "github.com/andygrunwald/go-jira"

// Issues is a list of issues which conveniently exposes a Print method to
// display the issue's data.
type Issues []Issue

// Sprints is a list of sprints which conveniently exposes a Print method to
// display sprints.
type Sprints []Sprint

// Issue is a Jira issue abstraction. The type primarily exists to only
// serialize a susbet of the data to disk.
type Issue struct {
	Key     string
	Summary string
}

// Sprint is a Jira sprint abstraction.  The type primarily exists to only
// serialize a susbet of the data to disk.
type Sprint struct {
	ID   int
	Name string
}

// NewIssues returns a new instance of Issues by converting jira.Issue to
// Issue.
func NewIssues(issues []jira.Issue) Issues {
	result := make(Issues, len(issues))
	for i, issue := range issues {
		result[i] = Issue{
			Key:     issue.Key,
			Summary: issue.Fields.Summary,
		}
	}
	return result
}

// NewSprints returns a new instance of Sprints by converting jira.Sprint to
// Sprint.
func NewSprints(sprints []jira.Sprint) Sprints {
	result := make(Sprints, len(sprints))
	for i, sprint := range sprints {
		result[i] = Sprint{
			ID:   sprint.ID,
			Name: sprint.Name,
		}
	}
	return result
}
