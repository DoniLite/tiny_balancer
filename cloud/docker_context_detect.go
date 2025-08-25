package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type dockerContextInfo struct {
	Name      string `json:"Name"`
	Endpoints struct {
		Docker struct {
			Host          string `json:"Host"`
			SkipTLSVerify bool   `json:"SkipTLSVerify"`
		} `json:"docker"`
	} `json:"Endpoints"`
}

// read active docker context name: `docker context show`
func currentDockerContext() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "context", "show")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	name := strings.TrimSpace(out.String())
	if name == "" {
		return "", errors.New("empty docker context name")
	}
	return name, nil
}

// read host from `docker context inspect <name>` JSON
func dockerHostFromContext(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "context", "inspect", name)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	var arr []dockerContextInfo
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		return "", err
	}
	if len(arr) == 0 {
		return "", errors.New("no context items")
	}
	host := strings.TrimSpace(arr[0].Endpoints.Docker.Host)
	if host == "" {
		return "", errors.New("no docker endpoint in context")
	}
	return host, nil
}

// Best-effort: figure out the docker host by asking the CLI (same as your terminal UX).
func resolveDockerHostViaCLI() (string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", err
	}
	// If DOCKER_CONTEXT is set, prefer it; else query active
	name := os.Getenv("DOCKER_CONTEXT")
	if strings.TrimSpace(name) == "" {
		var err error
		name, err = currentDockerContext()
		if err != nil {
			return "", err
		}
	}
	return dockerHostFromContext(name)
}
