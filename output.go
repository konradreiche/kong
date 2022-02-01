package kong

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// Print formats a list of issues and writes them to stdout.
func (i Issues) Print() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, issue := range i {
		fmt.Fprintf(w, "%s\t-\t%s\n", issue.Key, issue.Summary)
	}
	w.Flush()
}

// Print formats a list of issues with sprint status and writes them to stdout.
func (i Issues) PrintSprint() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, issue := range i {
		fmt.Fprintf(w, "%s\t-\t%s\n", issue.Key, issue.Summary)
	}
	w.Flush()
}

// Print formats a list of sprints and writes them to stdout.
func (s Sprints) Print() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, sprint := range s {
		fmt.Fprintf(w, "%d\t-\t%s\n", sprint.ID, sprint.Name)
	}
	w.Flush()
}
