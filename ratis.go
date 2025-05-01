package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type RatisNode struct {
	ID      string
	logger  *Logger
	process *exec.Cmd
	config  *NodeConfig

	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func NewRatisNode(config *NodeConfig, logger *Logger) *RatisNode {
	return &RatisNode{
		ID:      config.NodeId,
		logger:  logger,
		process: nil,
		config:  config,
		stdout:  nil,
		stderr:  nil,
	}
}

func (x *RatisNode) Create() {
	serverArgs := []string{
		x.config.LogConfig,
		"-cp",
		x.config.ServerPath,
		"org.apache.ratis.examples.counter.server.CounterServer",
		strconv.Itoa(x.config.ClusterID),
		strconv.Itoa(x.config.SchedulerPort),
		strconv.Itoa(x.config.InterceptorPort),
		x.config.NodeId,
		x.config.PeerAddresses,
		"02511d47-d67c-49a3-9011-abb3109a44c1", // TODO - May need to cycle
		"0",
	}
	// for i := 1; i <= x.config.NumNodes; i++ {
	// 	serverArgs = append(serverArgs, fmt.Sprintf("%d,localhost,%d", i, x.config.BaseGroupPort+i))
	// }
	x.logger.With(LogParams{"server-args": strings.Join(serverArgs, "")}).Debug("Creating server...")

	x.process = exec.Command("java", serverArgs...)
	x.process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if x.stdout == nil {
		x.stdout = new(bytes.Buffer)
	}
	if x.stderr == nil {
		x.stderr = new(bytes.Buffer)
	}
	x.process.Stdout = x.stdout
	x.process.Stderr = x.stderr
}

func (x *RatisNode) Start() error {
	x.logger.Debug("Starting node...")
	x.Create()
	if x.process == nil {
		return errors.New("ratis server not started")
	}
	return x.process.Start()
}

func (x *RatisNode) Cleanup() {
	os.RemoveAll(x.config.WorkDir)
}

func (x *RatisNode) Stop() error {
	x.logger.Debug("Stopping node...")
	if x.process == nil {
		return errors.New("ratis server not started")
	}
	// done := make(chan error, 1)
	// go func() {
	// 	err := x.process.Wait()
	// 	done <- err
	// }()

	// var err error = nil
	// select {
	// case <- time.After(50 * time.Millisecond):
	// 	err = x.process.Process.Kill()
	// case err = <- done:
	// }

	var err error
	if x.process.Process != nil {
		err = syscall.Kill(-x.process.Process.Pid, syscall.SIGKILL)
	}

	x.process = nil

	return err
}

func (x *RatisNode) GetLogs() (string, string) {
	if x.stdout == nil || x.stderr == nil {
		x.logger.Debug("Nil stdout or stderr.")
		return "", ""
	}

	return x.stdout.String(), x.stderr.String()
}

type RatisClient struct {
	ClientBinary     string
	logger           *Logger
	RatisLog4jConfig string
	PeerAddresses    string
}

func NewRatisClient(clientBinary, peerAddresses, log4jConfig string, logger *Logger) *RatisClient {
	return &RatisClient{
		ClientBinary:     clientBinary,
		logger:           logger,
		RatisLog4jConfig: log4jConfig,
		PeerAddresses:    peerAddresses,
	}
}

func (c *RatisClient) SendRequest() {
	c.logger.Debug("Sending client request...")
	clientArgs := []string{
		c.RatisLog4jConfig,
		"-cp",
		c.ClientBinary,
		"1",
		c.PeerAddresses,
		"02511d47-d67c-49a3-9011-abb3109a44c1", // TODO - May need it as param
	}
	// for i := 1; i <= c.NumNodes; i++ {
	// 	clientArgs = append(clientArgs, fmt.Sprintf("%d,localhost,%d", i, c.BaseServicePort+i))
	// }

	process := exec.Command("java", clientArgs...)

	// cmdDone := make(chan error, 1)
	process.Start()

	select {
	case <-time.After(2 * time.Second):
		syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
	default:
	}
}
