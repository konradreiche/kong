package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/konradreiche/kong"
	"github.com/spf13/cobra"
)

var (
	projectFlag string
	allFlag     bool
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
		d, err := kong.NewDaemon()
		if err != nil {
			exit(err)
		}
		d.Run(cmd.Context())
	},
}

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List and create issues",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		if projectFlag != "" {
			jira, err := kong.NewJira()
			if err != nil {
				exit(err)
			}
			issues, err := jira.ListIssues(ctx, projectFlag)
			if err != nil {
				exit(err)
			}
			issues.Print(cmd.OutOrStderr())
			return
		}
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		issues, err := data.GetIssues(ctx)
		if err != nil {
			exit(err)
		}
		issues.Print(cmd.OutOrStdout())
	},
}

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Perform actions on an issue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var editIssueCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit an existing issue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		editor, err := kong.NewEditor(ctx)
		if err != nil {
			exit(err)
		}
		must(editor.OpenEditIssueEditor(ctx, args[0]))
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
		must(editor.OpenNewIssueEditor(ctx))
	},
}

var epicsCmd = &cobra.Command{
	Use:   "epics",
	Short: "List epics",
	Run: func(cmd *cobra.Command, args []string) {
		if projectFlag != "" {
			jira, err := kong.NewJira()
			if err != nil {
				exit(err)
			}
			epics, err := jira.ListEpics(cmd.Context(), projectFlag)
			if err != nil {
				exit(err)
			}
			epics.Print(cmd.OutOrStderr())
			return
		}

		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		epics, err := data.GetEpics(cmd.Context())
		if err != nil {
			exit(err)
		}
		epics.Print(cmd.OutOrStderr())
	},
}

var newEpicsCmd = &cobra.Command{
	Use:   "new",
	Short: "Create new epics",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		editor, err := kong.NewEditor(ctx)
		if err != nil {
			exit(err)
		}
		must(editor.OpenEpicEditor(ctx))
	},
}

var initiativesCmd = &cobra.Command{
	Use:   "initiatives",
	Short: "List Initiatives",
	Run: func(cmd *cobra.Command, args []string) {
		if projectFlag != "" {
			jira, err := kong.NewJira()
			if err != nil {
				exit(err)
			}
			initiatives, err := jira.ListInitiatives(cmd.Context(), projectFlag)
			if err != nil {
				exit(err)
			}
			initiatives.Print(cmd.OutOrStderr())
			return
		}

		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		initiatives, err := data.GetInitiatives(cmd.Context())
		if err != nil {
			exit(err)
		}
		initiatives.Print(cmd.OutOrStderr())
	},
}

var sprintsCmd = &cobra.Command{
	Use:   "sprints",
	Short: "List and create sprints",
	Run: func(cmd *cobra.Command, args []string) {
		// request sprints if an alternative project is provided
		if projectFlag != "" {
			jira, err := kong.NewJira()
			if err != nil {
				exit(err)
			}
			boardID, err := jira.GetBoardID(projectFlag)
			if err != nil {
				exit(err)
			}
			sprints, err := jira.ListSprintsForBoard(boardID)
			if err != nil {
				exit(err)
			}
			sprints.Print()
			return
		}

		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		sprints, err := data.GetSprints(cmd.Context())
		if err != nil {
			exit(err)
		}
		sprints.Print()
	},
}

var newSprintCmd = &cobra.Command{
	Use:                   "new [name] [mm/dd]",
	Short:                 "Create a new sprint",
	Args:                  cobra.ExactArgs(2),
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

		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		jira, err := kong.NewJira()
		if err != nil {
			exit(err)
		}
		must(jira.CreateSprint(name, month, day, data.BoardID))
	},
}

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "List issues in current sprint",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		issues, err := data.GetSprintIssues(cmd.Context())
		if err != nil {
			exit(err)
		}
		issues.PrintSprint(allFlag)
	},
}

var editSprintCmd = &cobra.Command{
	Use:   "edit",
	Short: "Update sprint board issue progress",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		editor, err := kong.NewEditor(ctx)
		if err != nil {
			exit(err)
		}
		must(editor.OpenSprintEditor(ctx, allFlag))
	},
}

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Create a template-based Slack standup message",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		editor, err := kong.NewEditor(ctx)
		if err != nil {
			exit(err)
		}
		must(editor.OpenStandupEditor(ctx, args[0]))
	},
}

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Create a new branch named after the most recently created issue key",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		data, err := kong.LoadData()
		if err != nil {
			exit(err)
		}
		if data.LastIssueCreated == "" {
			exit(errors.New("no recently created issue"))
		}
		gitCmd := exec.CommandContext(ctx, "git", "checkout", "-b", data.LastIssueCreated)
		stdoutStderr, err := gitCmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(stdoutStderr))
			exit(err)
		}
		if len(stdoutStderr) > 0 {
			exit(errors.New(string(stdoutStderr)))
		}
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
		must(r.ReadString("Issue Type", &config.IssueType))
		must(r.ReadStringSlice("Labels", &config.Labels))
		must(r.ReadStringSlice("Components", &config.Components))

		must(r.ReadString("Sprint Keyword", &config.SprintKeyword))
		must(r.ReadInt("Sprint Duration (days)", &config.SprintDuration))

		must(r.ReadString("Epic Field", &config.CustomFields.Epics))
		must(r.ReadString("Sprint Field", &config.CustomFields.Sprints))
		must(r.ReadString("Story Points", &config.CustomFields.StoryPoints))

		must(r.ReadString("Copy Command", &config.CopyCommand))
		must(config.Write())
	},
}

// Execute assembles the all commands and sub-commands and executes the
// program.
func Execute() {
	// root coomands
	cmd.AddCommand(configureCmd)
	cmd.AddCommand(daemonCmd)
	cmd.AddCommand(initiativesCmd)
	cmd.AddCommand(standupCmd)
	cmd.AddCommand(branchCmd)

	// sprint command and sprint sub-commands
	cmd.AddCommand(sprintCmd)
	sprintCmd.AddCommand(editSprintCmd)

	// issues command and issues sub-commands
	cmd.AddCommand(issuesCmd)
	issuesCmd.AddCommand(newIssuesCmd)

	// issue command and issue sub-commands
	cmd.AddCommand(issueCmd)
	issueCmd.AddCommand(editIssueCmd)

	// epics and epics sub-commands
	cmd.AddCommand(epicsCmd)
	epicsCmd.AddCommand(newEpicsCmd)

	// sprints and sprints sub-commands
	cmd.AddCommand(sprintsCmd)
	sprintsCmd.AddCommand(newSprintCmd)

	// configure flags
	sprintCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Include issues that are done")

	for _, cmd := range []*cobra.Command{
		issuesCmd,
		epicsCmd,
		initiativesCmd,
		sprintsCmd,
	} {
		cmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Reference alternative project")
	}

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
