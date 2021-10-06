package kong

import (
	"reflect"
	"testing"
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
