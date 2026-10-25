package ai

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemSecrets keeps credentials outside the repository and preferences JSON.
// Secret values are supplied via stdin, never command arguments.
type SystemSecrets struct{}

func (SystemSecrets) Available() bool {
	name := ""
	switch runtime.GOOS {
	case "darwin":
		name = "security"
	case "linux":
		name = "secret-tool"
	default:
		return false
	}
	_, err := exec.LookPath(name)
	return err == nil
}
func (SystemSecrets) Get(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "security", "find-generic-password", "-s", "partyhub.ai", "-a", id, "-w")
	case "linux":
		cmd = exec.CommandContext(ctx, "secret-tool", "lookup", "service", "partyhub.ai", "account", id)
	default:
		return "", errors.New("secure storage unavailable")
	}
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("credential lookup failed")
	}
	return strings.TrimSpace(string(output)), nil
}
func (SystemSecrets) Set(id, key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "security", "-i")
		cmd.Stdin = strings.NewReader("add-generic-password -U -s partyhub.ai -a " + strconv.Quote(id) + " -w " + strconv.Quote(key) + "\n")
	case "linux":
		cmd = exec.CommandContext(ctx, "secret-tool", "store", "--label=Party Hub AI", "service", "partyhub.ai", "account", id)
		cmd.Stdin = strings.NewReader(key)
	default:
		return errors.New("secure storage unavailable")
	}
	if err := cmd.Run(); err != nil {
		return errors.New("credential storage failed")
	}
	stored, err := (SystemSecrets{}).Get(id)
	if err != nil || stored != key {
		return errors.New("credential storage could not be verified")
	}
	return nil
}
func (SystemSecrets) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "security", "delete-generic-password", "-s", "partyhub.ai", "-a", id)
	case "linux":
		cmd = exec.CommandContext(ctx, "secret-tool", "clear", "service", "partyhub.ai", "account", id)
	default:
		return errors.New("secure storage unavailable")
	}
	if err := cmd.Run(); err != nil {
		return errors.New("credential deletion failed")
	}
	return nil
}
