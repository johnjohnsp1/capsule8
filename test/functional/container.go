package functional

import (
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/net/context"
)

type Container struct {
	t       *testing.T
	Path    string
	ImageID string
	command *exec.Cmd
}

func (c *Container) Build() error {
	docker := exec.Command("docker", "build", c.Path)
	err := docker.Run()
	if err != nil {
		return err
	}

	docker = exec.Command("docker", "build", "-q", c.Path)
	dockerOutput, err := docker.Output()
	if err != nil {
		return err
	}

	c.ImageID = strings.TrimSpace(string(dockerOutput))

	return nil
}

func (c *Container) Start() error {
	c.command = exec.Command("docker", "run", "--rm", c.ImageID)
	return c.command.Start()
}

func (c *Container) StartContext(ctx context.Context) error {
	c.command = exec.CommandContext(ctx, "docker", "run", "--rm", c.ImageID)
	return c.command.Start()
}

func (c *Container) Wait() error {
	return c.command.Wait()
}

func (c *Container) Run() error {
	c.command = exec.Command("docker", "run", "--rm", c.ImageID)
	return c.command.Run()
}

func (c *Container) RunContext(ctx context.Context) error {
	c.command = exec.CommandContext(ctx, "docker", "run", "--rm", c.ImageID)
	return c.command.Run()
}

func NewContainer(t *testing.T, path string) *Container {
	return &Container{
		t:    t,
		Path: path,
	}
}
