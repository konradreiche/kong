package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/konradreiche/kong"
)

func TestIssueCommand(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "kong-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(f.Name()); err != nil {
			t.Fatal(err)
		}
	})
	setEnvironmentVariable(t, f.Name())

	data := kong.Data{
		Timestamp: time.Now().Unix(),
		Issues: kong.Issues{
			{
				Key:     "KONG-1",
				Summary: "Add command to list issues",
				Status: kong.Status{
					Name: "To Do",
				},
			},
		},
	}
	if err := data.WriteFile(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	issuesCmd.SetOut(&buf)

	if err := issuesCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	want := "KONG-1 - To Do - Add command to list issues\n"
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("diff: %s", diff)
	}
}

func setEnvironmentVariable(t *testing.T, value string) {
	t.Helper()

	// check if environment variable is set to determine cleanup behavior
	env, ok := os.LookupEnv("KONG_CACHE")
	if err := os.Setenv("KONG_CACHE", "test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !ok {
			if err := os.Unsetenv("KONG_CACHE"); err != nil {
				t.Fatal(err)
			} else {
				if err := os.Setenv("KONG_CACHE", env); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := os.Setenv("KONG_CACHE", value); err != nil {
			t.Fatal(err)
		}
	})
}
