package kong

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// Print formats a list of issues and writes them to stdout.
func (i Issues) Print() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, issue := range i.Sort() {
		fmt.Fprintf(w, "%s\t-\t%s\t-\t%s\n", issue.Key, issue.Status.Name, issue.Summary)
	}
	w.Flush()
}

// Print formats a list of issues with sprint status and writes them to stdout.
func (i Issues) PrintSprint() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, issue := range i.Sort() {
		fmt.Fprintf(w, "%s\t-\t%s\t-\t%s\n", issue.Status.Name, issue.Key, issue.Summary)
	}
	w.Flush()
}

// Print formats a list of sprints and writes them to stdout.
func (s Sprints) Print() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, sprint := range s {
		endDate := "N/A"
		if !sprint.EndDate.IsZero() {
			endDate = sprint.EndDate.Local().Format("2006/1/2")
		}
		fmt.Fprintf(w, "%d\t-\t%s\t-\t%s\n", sprint.ID, endDate, sprint.Name)
	}
	w.Flush()
}
