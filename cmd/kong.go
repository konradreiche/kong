package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/konradreiche/kong"
	"github.com/spf13/cobra"
)

func main() {
	Execute()
}

var cmd = &cobra.Command{
	Use:   "kong",
	Short: "🦍 Kong is a Jira CLI for low-latency workflows",
	Run: func(cmd *cobra.Command, args []string) {
		must(cmd.Help())
	},
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run background process",
	Run: func(cmd *cobra.Command, args []string) {
		jira, err := kong.NewJira()
		if err != nil {
			exit(err)
		}
		kong.NewDaemon(jira).Run()
	},
}

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List and create issues",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		issues, err := data.GetIssues()
		if err != nil {
			exit(err)
		}
		issues.Print()
	},
}

var newIssuesCmd = &cobra.Command{
	Use:   "new",
	Short: "Create new issues",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		editor, err := kong.NewEditor(ctx)
		if err != nil {
			exit(err)
		}
		must(editor.OpenIssueEditor(ctx))
	},
}

var sprintIssuesCmd = &cobra.Command{
	Use:   "sprint",
	Short: "List issues in current sprint",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		issues, err := data.GetSprintIssues()
		if err != nil {
			exit(err)
		}
		issues.Print()
	},
}

var epicsCmd = &cobra.Command{
	Use:   "epics",
	Short: "List epics",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		epics, err := data.GetEpics()
		if err != nil {
			exit(err)
		}
		epics.Print()
	},
}

var sprintsCmd = &cobra.Command{
	Use:   "sprints",
	Short: "List and create sprints",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		sprints, err := data.GetSprints()
		if err != nil {
			exit(err)
		}
		sprints.Print()
	},
}

var newSprintCmd = &cobra.Command{
	Use:                   "new [name] [mm/dd]",
	Short:                 "Create a new sprint",
	Args:                  cobra.MinimumNArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		date := strings.Split(args[1], "/")
		if len(date) != 2 {
			exitPrompt("Error: requires month and day")
		}

		month, err := strconv.Atoi(date[0])
		if err != nil {
			exitPrompt("Error: month has to be numeric")
		}
		day, err := strconv.Atoi(date[1])
		if err != nil {
			exitPrompt("Error: day has to be numeric")
		}

		jira, err := kong.NewJira()
		if err != nil {
			exit(err)
		}
		must(jira.CreateSprint(name, month, day))
	},
}

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "configure",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := kong.LoadConfig()
		if err != nil && err != kong.ErrConfigMissing {
			exit(err)
		}

		fmt.Println("Configure Jira using basic authentication.")

		r := kong.NewConfigReader()
		must(r.ReadString("Endpoint", &config.Endpoint))
		must(r.ReadString("Username", &config.Username))
		must(r.ReadString("Password", &config.Password))

		must(r.ReadString("Project", &config.Project))
		must(r.ReadInt("Board ID", &config.BoardID))
		must(r.ReadString("Issue Type", &config.IssueType))
		must(r.ReadStringSlice("Labels", &config.Labels))
		must(r.ReadStringSlice("Components", &config.Components))

		must(r.ReadString("Sprint Keyword", &config.SprintKeyword))
		must(r.ReadInt("Sprint Duration (days)", &config.SprintDuration))

		must(r.ReadString("Epic Field", &config.CustomFields.Epics))
		must(r.ReadString("Sprint Field", &config.CustomFields.Sprints))
		must(r.ReadString("Story Points", &config.CustomFields.StoryPoints))
		must(config.Write())
	},
}

func Execute() {
	cmd.AddCommand(configureCmd)
	cmd.AddCommand(daemonCmd)
	cmd.AddCommand(epicsCmd)

	cmd.AddCommand(issuesCmd)
	issuesCmd.AddCommand(newIssuesCmd)
	issuesCmd.AddCommand(sprintIssuesCmd)

	cmd.AddCommand(sprintsCmd)
	sprintsCmd.AddCommand(newSprintCmd)

	if err := cmd.Execute(); err != nil {
		exit(err)
	}
}

func must(err error) {
	if err == nil {
		return
	}
	exit(err)
}

func exit(err error) {
	if err == kong.ErrConfigMissing {
		exitPrompt("Configuration is missing. Run kong configure")
	}
	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	os.Exit(1)
}

func exitPrompt(prompt string) {
	fmt.Fprintf(os.Stderr, "%s\n", prompt)
	os.Exit(1)
}
