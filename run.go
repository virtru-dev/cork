package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/satori/go.uuid"

	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
)

// CorkDefinition - The Cork Definition file
type CorkDefinition struct {
	Name string `yaml:"name,omitempty"`
	Type string `yaml:"type"`
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

	// If no project name is defined use the base directory name
	if corkDef.Name == "" {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		corkDef.Name = path.Base(dir)
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

func cmdRun(c *cli.Context) error {
	control := NewControl()
	control.HandleTerminate()

	log.Debug("Loading cork.yml")
	corkDef, err := loadCorkYaml()
	if err != nil {
		return err
	}

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

	log.Debug("Initializing runner")
	runner, err := New(corkDef.Name, client, corkDef.Type, metadata.CacheVolumeName(), control)
	if err != nil {
		return err
	}

	stageName := c.Args().Get(0)
	if stageName == "" {
		stageName = "default"
	}

	err = runner.Start(stageName)
	if err != nil {
		if !(strings.Contains(err.Error(), "without exit status") && control.Terminating) {
			return err
		}
	}
	return nil
}
