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

	trimmed := strings.TrimSpace(string(dockerOutput))
	c.ImageID = strings.TrimPrefix(trimmed, "sha256:")

	return nil
}

func (c *Container) dockerRunArgs(runargs []string) []string {
	args := append([]string{"run", "--rm"}, runargs...)
	args = append(args, c.ImageID)
	return args
}

func (c *Container) Start(runargs ...string) error {
	dockerArgs := c.dockerRunArgs(runargs)
	c.command = exec.Command("docker", dockerArgs...)
	return c.command.Start()
}

func (c *Container) StartContext(ctx context.Context, runargs ...string) error {
	dockerArgs := c.dockerRunArgs(runargs)
	c.command = exec.CommandContext(ctx, "docker", dockerArgs...)
	return c.command.Start()
}

func (c *Container) Wait() error {
	return c.command.Wait()
}

func (c *Container) Run(runargs ...string) error {
	dockerArgs := c.dockerRunArgs(runargs)
	c.command = exec.Command("docker", dockerArgs...)
	return c.command.Run()
}

func (c *Container) RunContext(ctx context.Context, runargs ...string) error {
	dockerArgs := c.dockerRunArgs(runargs)
	c.command = exec.CommandContext(ctx, "docker", dockerArgs...)
	return c.command.Run()
}

func NewContainer(t *testing.T, path string) *Container {
	return &Container{
		t:    t,
		Path: path,
	}
}
