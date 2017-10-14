package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/satori/go.uuid"

	"github.com/fatih/color"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
)

// CorkDefinition - The Cork Definition file
type CorkDefinition struct {
	Name   string            `yaml:"name,omitempty"`
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params,omitempty"`
}

func (cd *CorkDefinition) LoadName() error {
	// If no project name is defined use the base directory name
	if cd.Name == "" {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		cd.Name = path.Base(dir)
	}
	return nil
}

// CorkProjectMetadata - used to store metadata about the current project
type CorkProjectMetadata struct {
	ID string `json:"id"`
}

func (cpm *CorkProjectMetadata) CacheVolumeName() string {
	return fmt.Sprintf("cork-cache-%s", cpm.ID)
}

func init() {
	command := cli.Command{
		Name:        "run",
		Description: "Determine available commands",
		Action:      cmdRun,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:   "force-pull-image",
				Usage:  "Forces cork to pull the latest version of the cork container",
				EnvVar: "CORK_FORCE_PULL_IMAGE",
			},
			cli.StringFlag{
				Name:   "ssh-key",
				Usage:  "The ssh key path to use",
				EnvVar: "CORK_SSH_KEY",
			},
			cli.StringFlag{
				Name:   "output, o",
				Usage:  "The path to the output destination",
				EnvVar: "CORK_OUTPUT_DESTINATION",
				Value:  "outputs.json",
			},
			cli.StringSliceFlag{
				Name:  "param, p",
				Usage: `Set Paramater "param_name=param_value"`,
			},
		},
	}
	registerCommand(command)
}

func loadCorkYaml() (*CorkDefinition, error) {
	file, err := os.Open("cork.yml")
	if err != nil {
		return nil, fmt.Errorf("Cannot properly read the cork.yml file. Does it exist?")
	}
	defer file.Close()

	var corkDef CorkDefinition
	corkDefBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(corkDefBytes, &corkDef)
	if err != nil {
		return nil, err
	}

	if corkDef.Type == "" {
		return nil, fmt.Errorf("cork.yml has no type defined. Cannot continue")
	}

	err = corkDef.LoadName()
	if err != nil {
		return nil, err
	}

	return &corkDef, nil
}

type Control struct {
	Kill             chan os.Signal
	Done             chan bool
	ChildKillSignals []chan bool
	ChildrenCount    int
	Terminating      bool
}

// Add a child
func (c *Control) AddChild() chan bool {
	c.ChildrenCount++
	childKillSignal := make(chan bool)
	c.ChildKillSignals = append(c.ChildKillSignals, childKillSignal)
	return childKillSignal
}

func (c *Control) NotifyAll() {
	for _, childKillSignal := range c.ChildKillSignals {
		childKillSignal <- true
	}
}

func (c *Control) WaitForAll() {
	for i := 0; i < c.ChildrenCount; i++ {
		<-c.Done
	}
}

// OnTerminate - Terminates
func (c *Control) OnTerminate(clean func()) {
	childKillSignal := c.AddChild()
	go func() {
		<-childKillSignal
		clean()
		c.Done <- true
	}()
}

func NewControl() *Control {
	killChan := make(chan os.Signal, 2)
	doneChan := make(chan bool)
	return &Control{
		Kill:          killChan,
		Done:          doneChan,
		ChildrenCount: 0,
	}
}

func (c *Control) HandleTerminate() {
	signal.Notify(c.Kill, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c.Kill
		c.Terminating = true
		c.NotifyAll()
		c.WaitForAll()
		os.Exit(1)
	}()
}

func loadCorkProjectMetadata() (*CorkProjectMetadata, error) {
	metadataDir := ".cork"
	metadataJSONPath := path.Join(metadataDir, "metadata.json")
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		os.Mkdir(metadataDir, 0700)
	}

	if _, err := os.Stat(metadataJSONPath); os.IsNotExist(err) {
		uuidStr := uuid.NewV4().String()
		metadata := CorkProjectMetadata{
			ID: uuidStr,
		}
		metadataJSONBytes, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(metadataJSONPath, metadataJSONBytes, 0700)
		if err != nil {
			return nil, err
		}
		return &metadata, nil
	}

	metadataJSONBytes, err := ioutil.ReadFile(metadataJSONPath)
	if err != nil {
		return nil, err
	}

	var metadata CorkProjectMetadata
	err = json.Unmarshal(metadataJSONBytes, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

func executeCorkRun(c *cli.Context, corkDef *CorkDefinition, stageName string) error {
	control := NewControl()
	control.HandleTerminate()

	log.Debugf("Loading cork metadata for project %s", corkDef.Name)
	metadata, err := loadCorkProjectMetadata()
	if err != nil {
		return err
	}

	log.Debug("Connecting to docker")
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return err
	}

	params := c.StringSlice("param")

	if corkDef.Params == nil {
		corkDef.Params = make(map[string]string)
	}

	for _, rawParam := range params {
		splitParams := strings.Split(rawParam, "=")

		paramName := splitParams[0]
		paramValue := strings.Join(splitParams[1:], "=")

		corkDef.Params[paramName] = paramValue
	}

	cliOutput := c.String("output")
	if cliOutput == "" {
		cliOutput = "outputs.json"
	}

	outputDestinationPath, err := filepath.Abs(cliOutput)
	if err != nil {
		return err
	}

	options := CorkTypeContainerOptions{
		ProjectName:           corkDef.Name,
		CacheVolumeName:       metadata.CacheVolumeName(),
		ImageName:             corkDef.Type,
		Debug:                 c.GlobalBool("debug"),
		ForcePullImage:        c.Bool("force-pull-image"),
		SSHKeyPath:            c.String("ssh-key"),
		Definition:            corkDef,
		OutputDestinationPath: outputDestinationPath,
	}

	log.Debug("Initializing runner")
	runner, err := New(client, control, options)
	if err != nil {
		return err
	}

	blue := color.New(color.FgBlue)
	black := color.New(color.FgBlack)

	fmt.Printf("Cork - The most reliable build tool ever conceived! (... probably)\n\n")

	blue.Printf("Cork Is Running\n")
	blue.Printf("-------------------\n")
	blue.Printf("Project: ")
	black.Printf("%s\n", corkDef.Name)
	blue.Printf("Project Type: ")
	black.Printf("%s\n", corkDef.Type)
	blue.Printf("Executing Stage: ")
	black.Printf("%s\n", stageName)
	blue.Printf("-------------------\n")

	err = runner.Start(stageName)
	if control.Terminating {
		color.Red("\nCork run terminated")
	}
	if err != nil {
		if !(strings.Contains(err.Error(), "without exit status") && control.Terminating) {
			color.Red("\nCork failed")

			if strings.Contains(err.Error(), "InitializationError") {
				fmt.Println("")
				fmt.Println("")
				color.Red("======== ERROR HELP ========")
				color.Red("Failed to initialize the cork server.")
				fmt.Println("")
				color.Red("The detected error usually relates to a broken startup hook. Try setting `cork --debug`")
				color.Red("for more information.")
			}

			if strings.Contains(err.Error(), "CannotRunSSHCommand") {
				fmt.Println("")
				fmt.Println("")
				color.Red("======== ERROR HELP ========")
				color.Red("Failed to connect to the cork server.")
				fmt.Println("")
				color.Red("This is done through ssh and, for now, requires an ssh agent to be configured")
				color.Red("with the appropriate key. Try setting `cork --debug` for more information.")
			}
			log.Errorf("%v", err)
			os.Exit(1)
		}
	}
	color.Green("\nCork is done!")
	color.Green("Find your outputs: %s", outputDestinationPath)
	return nil
}

func cmdRun(c *cli.Context) error {
	log.Debug("Loading cork.yml")
	corkDef, err := loadCorkYaml()
	if err != nil {
		return err
	}

	stageName := c.Args().Get(0)
	if stageName == "" {
		stageName = "default"
	}

	err = executeCorkRun(c, corkDef, stageName)
	if err != nil {
		return err
	}
	return nil
}
