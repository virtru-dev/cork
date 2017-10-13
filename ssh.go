package main

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"

	"io"

	"crypto/x509"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var maxRetries = 15

type DockerSSHCommand struct {
	Host            string
	Port            int
	Command         string
	Failed          chan bool
	SSHKeyPath      string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	SSHAgentManager SSHAgentManager
}

type DockerSSHCommandOptions struct {
	Failed     chan bool
	Command    string
	Host       string
	Port       int
	SSHKeyPath string
}

type SSHAgentManager interface {
	Shutdown() error
	GetSSHAgent() agent.Agent
	Forward(client *ssh.Client) error
}

type SystemSSHAgentManager struct {
	Connection      net.Conn
	Agent           agent.Agent
	SSHAuthSockPath string
}

func NewSystemSSHAgentManager(expectedSSHKeyPath string, sshAuthSockPath string) (SSHAgentManager, error) {
	sshAgent, err := net.Dial("unix", sshAuthSockPath)
	if err != nil {
		return nil, err
	}

	agentClient := agent.NewClient(sshAgent)
	agentKeys, err := agentClient.List()
	if err != nil {
		return nil, err
	}

	found := false
	for _, key := range agentKeys {
		if key.Comment == expectedSSHKeyPath {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("CannotRunSSHCommand: Could not find appropriate key")
	}
	return &SystemSSHAgentManager{
		Agent:           agentClient,
		Connection:      sshAgent,
		SSHAuthSockPath: sshAuthSockPath,
	}, nil
}

func (s *SystemSSHAgentManager) Shutdown() error {
	return s.Connection.Close()
}

func (s *SystemSSHAgentManager) GetSSHAgent() agent.Agent {
	return s.Agent
}

func (s *SystemSSHAgentManager) Forward(client *ssh.Client) error {
	return agent.ForwardToRemote(client, s.SSHAuthSockPath)
}

type InMemorySSHAgentManager struct {
	Agent agent.Agent
}

func NewInMemorySSHAgentManager(pathToPrivateKey string) (SSHAgentManager, error) {
	privateKeyPemBytes, err := ioutil.ReadFile(pathToPrivateKey)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(privateKeyPemBytes)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	addedKey := agent.AddedKey{
		PrivateKey: privateKey,
		Comment:    pathToPrivateKey,
	}

	agentKeyring := agent.NewKeyring()
	agentKeyring.Add(addedKey)

	return &InMemorySSHAgentManager{
		Agent: agentKeyring,
	}, nil
}

func (i *InMemorySSHAgentManager) Shutdown() error {
	return nil
}

func (i *InMemorySSHAgentManager) GetSSHAgent() agent.Agent {
	return i.Agent
}

func (i *InMemorySSHAgentManager) Forward(client *ssh.Client) error {
	return agent.ForwardToAgent(client, i.Agent)
}

const DefaultSSHPath = ".ssh/id_rsa"

func NewDockerSSHCommand(options DockerSSHCommandOptions) (*DockerSSHCommand, error) {
	log.Debugf("Creating NewDockerSSHCommand")
	// FIXME: We should have an SSH Search Path
	if options.SSHKeyPath == "" {
		log.Debugf("Loading current user")
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		options.SSHKeyPath = path.Join(usr.HomeDir, DefaultSSHPath)
	}

	sshCommand := &DockerSSHCommand{
		Host:       options.Host,
		Port:       options.Port,
		Command:    options.Command,
		Failed:     options.Failed,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		SSHKeyPath: options.SSHKeyPath,
	}

	err := sshCommand.ChooseSSHAgentManager()
	if err != nil {
		return nil, err
	}
	return sshCommand, nil
}

func (d *DockerSSHCommand) Start(envVars []string) {
	// Connect to the ssh
	go func() {
		log.SetLevel(log.DebugLevel)
		log.Debug("Starting the docker ssh command")
		err := d.runCommand(envVars)
		if err != nil {
			log.Errorf("Error running ssh command: %v", err)
		}
		d.Failed <- err != nil
	}()
}

func (d *DockerSSHCommand) connect() (*ssh.Client, error) {
	var err error
	authMethod, err := d.getAuthMethod()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{authMethod},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	hostStr := fmt.Sprintf("%s:%d", d.Host, d.Port)

	log.Debugf("Connecting to host: %s", hostStr)
	for i := 0; i < maxRetries; i++ {
		connection, err := ssh.Dial("tcp", hostStr, config)
		if err == nil {
			return connection, nil
		}
		time.Sleep(1 * time.Second)
		log.Debugf("Retrying connection to host: %s", hostStr)
	}
	log.Fatalf("Failed to connect to host: %s", hostStr)
	return nil, fmt.Errorf("Failed to connect on ssh: %s", err)
}

func (d *DockerSSHCommand) newSession() (*ssh.Session, error) {
	connection, err := d.connect()
	if err != nil {
		return nil, err
	}

	log.Debug("Setting up forwarding for the ssh connection")
	// Setup agent forwarding
	err = d.SSHAgentManager.Forward(connection)
	if err != nil {
		return nil, fmt.Errorf("Failed to forward agent: %s", err)
	}

	log.Debug("Creating ssh session")
	session, err := connection.NewSession()
	if err != nil {
		return nil, fmt.Errorf("Failed to create session: %s", err)
	}

	log.Debug("Setting up forwarding for the ssh session")
	// Setup forwarding for this session
	err = agent.RequestAgentForwarding(session)
	if err != nil {
		return nil, fmt.Errorf("Failed to create session: %s", err)
	}

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	log.Debug("Setting up PTY to get output")
	if err = session.RequestPty("xterm", 80, 40, modes); err != nil {
		session.Close()
		return nil, fmt.Errorf("request for pseudo terminal failed: %s", err)
	}
	return session, nil
}

func (d *DockerSSHCommand) runCommand(envVars []string) error {
	session, err := d.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	/*
	   for _, envVar := range envVars {
	       splitEnvVar := strings.Split(envVar, "=")
	       if len(splitEnvVar) < 2 {
	           return fmt.Errorf("Invalid env var `%s`", envVar)
	       }
	       envKey := splitEnvVar[0]
	       envValue := strings.Join(splitEnvVar[1:], "=")
	       log.Debugf("Setting `%s=%s` on ssh env", envKey, envValue)
	       err = session.Setenv(envKey, envValue)
	       if err != nil {
	           return err
	       }
	   }
	*/

	if d.Stdin != nil {
		stdin, err := session.StdinPipe()
		if err != nil {
			return fmt.Errorf("Unable to setup stdin for session: %v", err)
		}
		go io.Copy(stdin, d.Stdin)
	}

	if d.Stdout != nil {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return fmt.Errorf("Unable to setup stdout for session: %v", err)
		}
		go io.Copy(d.Stdout, stdout)
	}

	if d.Stderr != nil {
		stderr, err := session.StderrPipe()
		if err != nil {
			return fmt.Errorf("Unable to setup stderr for session: %v", err)
		}
		go io.Copy(d.Stderr, stderr)
	}

	return session.Run(d.Command)
}

func (d *DockerSSHCommand) CleanUp() error {
	return d.SSHAgentManager.Shutdown()
}

func (d *DockerSSHCommand) ChooseSSHAgentManager() error {
	log.Debugf("Choosing agent based on capabilities")

	// First check if there is an existing agent
	// If not check if there's an ssh key on the system at ~/.ssh/id_rsa
	// fail if that key is encrypted.

	sshAuthSockPath := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSockPath != "" {
		agentManager, err := NewSystemSSHAgentManager(d.SSHKeyPath, sshAuthSockPath)
		if err == nil {
			d.SSHAgentManager = agentManager
			return nil
		}
	}
	agentManager, err := NewInMemorySSHAgentManager(d.SSHKeyPath)
	if err != nil {
		return fmt.Errorf("Error choosing ssh agent. If your ssh key is passphrase protected you will need to start the ssh-agent: %+v", err)
	}
	d.SSHAgentManager = agentManager
	return nil
}

func (d *DockerSSHCommand) getAuthMethod() (ssh.AuthMethod, error) {
	log.Debug("Attempting to connect to ssh agent")
	return ssh.PublicKeysCallback(d.SSHAgentManager.GetSSHAgent().Signers), nil
}
