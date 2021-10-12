package kong

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/andygrunwald/go-jira"
)

func TestParseColumns(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  [][]string
		err   error
	}{
		{
			name: "no-comma-in-description",
			lines: []string{
				"3,1,foo,5.0,Lorem ipsum dolor sit amet consectetur",
			},
			want: [][]string{
				{
					"3",
					"1",
					"foo",
					"5.0",
					"Lorem ipsum dolor sit amet consectetur",
				},
			},
		},
		{
			name: "one-comma-in-description",
			lines: []string{
				"3,1,foo,5.0,Lorem ipsum dolor sit amet, consectetur",
			},
			want: [][]string{
				{
					"3",
					"1",
					"foo",
					"5.0",
					"Lorem ipsum dolor sit amet, consectetur",
				},
			},
		},
		{
			name: "trailign-comma-in-description",
			lines: []string{
				"3,1,foo,5.0,Lorem ipsum dolor sit amet, consectetur,",
			},
			want: [][]string{
				{
					"3",
					"1",
					"foo",
					"5.0",
					"Lorem ipsum dolor sit amet, consectetur,",
				},
			},
		},
		{
			name: "missing-columns",
			lines: []string{
				"3,1,foo,5.0",
			},
			err: errMissingColumn,
		},
	}

	for _, tt := range tests {
		got, err := Editor{}.parseColumns(tt.lines)
		if tt.err == nil && err != nil {
			t.Fatal(err)
		}
		if tt.err != nil && err != tt.err {
			t.Fatalf("got %v, want: %v", err, tt.err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("got %v, want: %v", got, tt.want)
		}
	}
}

func TestParseIssue(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		data    Data
		want    *jira.Issue
		err     error
	}{
		{
			name: "fails-non-integer-epics",
			columns: []string{
				"foo",
			},
			want: nil,
			err:  strconv.ErrSyntax,
		},
		{
			name: "fails-non-integer-sprints",
			columns: []string{
				"1",
				"foo",
			},
			want: nil,
			err:  strconv.ErrSyntax,
		},
		{
			name: "fails-non-number-story-points",
			columns: []string{
				"1",
				"2",
				"summary",
				"foo",
			},
			want: nil,
			err:  strconv.ErrSyntax,
		},
		{
			name: "fails-negative-epics",
			columns: []string{
				"-1",
				"2",
				"summary",
				"0.5",
				"description",
			},
			want: nil,
			err:  errEpicMismatch,
		},
		{
			name: "fails-missing-epics",
			columns: []string{
				"1",
				"2",
				"summary",
				"0.5",
				"description",
			},
			want: nil,
			err:  errEpicMismatch,
		},
		{
			name: "fails-missing-sprints",
			columns: []string{
				"1",
				"2",
				"summary",
				"0.5",
				"description",
			},
			data: Data{
				Epics: []Issue{
					{
						Key: "KONG-1",
					},
				},
			},
			want: nil,
			err:  errSprintMismatch,
		},
	}

	for _, tt := range tests {
		editor := Editor{
			data: tt.data,
		}

		got, err := editor.parseIssue(tt.columns)
		if tt.err == nil && err != nil {
			t.Fatal(err)
		}

		if tt.err != nil && !errors.Is(err, tt.err) {
			t.Fatalf("got %v, want: %v", err, tt.err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("got %v, want: %v", got, tt.want)
		}
	}
}
