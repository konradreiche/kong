package kong

import (
	"testing"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/google/go-cmp/cmp"
)

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
