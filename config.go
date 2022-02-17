package kong

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config provides the configuration for the Jira client. The configuration is
// used to authenticate the Jira client and customize Jira queries.
type Config struct {
	Endpoint string `yaml:"endpoint"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	Project      string       `yaml:"project"`
	IssueType    string       `yaml:"issueType"`
	Labels       []string     `yaml:"labels"`
	Components   []string     `yaml:"components"`
	CustomFields CustomFields `yaml:"customFields"`

	SprintKeyword  string `yaml:"sprintKeyword"`
	SprintDuration int    `yaml:"sprintDuration"`
}

// CustomFields provides configuration of custom fields to map fields like
// epics, sprints and story points to the Jira backend.
type CustomFields struct {
	Epics       string `yaml:"epics"`
	Sprints     string `yaml:"sprints"`
	StoryPoints string `yaml:"storyPoints"`
}

// Write ensures the configuration directory exists and writes the content of
// Config into a file for subsequent retrieval.
func (c Config) Write() (err error) {
	err = os.MkdirAll(c.dir(), os.ModePerm)
	if err != nil {
		return err
	}
	f, err := os.Create(c.filepath())
	if err != nil {
		return err
	}
	defer func() {
		err = f.Close()
	}()
	encoder := yaml.NewEncoder(f)
	err = encoder.Encode(c)
	if err != nil {
		return err
	}
	return encoder.Close()
}

// LoadConfig reads the current configuration from disk or returns an empty one
// if the file does not exist yet.
func LoadConfig() (Config, error) {
	var config Config
	if config.isMissing() {
		return config, ErrConfigMissing
	}
	b, err := os.ReadFile(config.filepath())
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(b, &config)
	return config, err
}

func (c Config) dir() string {
	return path.Join(os.Getenv("HOME"), ".config")
}

func (c Config) filepath() string {
	return path.Join(c.dir(), "kong")
}

func (c Config) isMissing() bool {
	_, err := os.Stat(c.filepath())
	return os.IsNotExist(err)
}

// Configurer parses the user input to configure Kong for Jira.
type Configurer struct {
	r *bufio.Reader
}

// NewConfigReader returns a new instance of Configurer used to parse user
// input for generating a Kong Jira configuration.
func NewConfigReader() Configurer {
	return Configurer{
		r: bufio.NewReader(os.Stdin),
	}
}

// ReadString prompts the user to enter a value prefixed with a given label and
// writes the user input to target.
func (c Configurer) ReadString(label string, target *string) error {
	var current string
	if *target != "" {
		current = " [" + *target + "]"
	}
	fmt.Print(label + current + ": ")
	s, err := c.r.ReadString('\n')
	if err != nil {
		return err
	}
	s = strings.TrimSuffix(s, "\n")

	// onyl write to target if user did not skip
	if s != "" {
		*target = s
	}
	return nil
}

// ReadInt prompts the user to enter a value prefixed with a given label and
// writes the user input to target.
func (c Configurer) ReadInt(label string, target *int) error {
	var s string
	if *target != 0 {
		s = fmt.Sprint(*target)
	}
	err := c.ReadString(label, &s)
	if err != nil {
		return err
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*target = int(i)
	return nil
}

// ReadStringSlice prompts the user to enter a comma-separated value prefixed
// with a given label and writes the user input to target.
func (c Configurer) ReadStringSlice(label string, target *[]string) error {
	var s string
	if target != nil {
		s = strings.Join(*target, ",")
	}
	err := c.ReadString(label, &s)
	if err != nil {
		return err
	}
	*target = strings.Split(s, ",")
	return nil
}
