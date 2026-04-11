package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Config struct {
	Container string
	User      string
	Password  string
	Database  string
}

func CheckContainerRunning(ctx context.Context, container string) error {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", container).CombinedOutput()
	if err != nil {
		return fmt.Errorf("container %q not found or docker not available: %w", container, err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("container %q is not running", container)
	}
	return nil
}

// dockerExecArgs returns the prefix: docker exec -i [-e MYSQL_PWD=...] <container>
func dockerExecArgs(cfg Config) []string {
	args := []string{"exec", "-i"}
	if cfg.Password != "" {
		args = append(args, "-e", "MYSQL_PWD="+cfg.Password)
	}
	args = append(args, cfg.Container)
	return args
}

// mysqlBaseArgs returns mysql -u <user> [-p] suitable for MYSQL_PWD when -p is present.
func mysqlBaseArgs(cfg Config) []string {
	out := []string{"mysql", "-u", cfg.User}
	if cfg.Password != "" {
		out = append(out, "-p")
	}
	return out
}

// DropAndCreateDB drops the target database and recreates it (no shell; password via MYSQL_PWD).
func DropAndCreateDB(ctx context.Context, cfg Config) error {
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	sql := fmt.Sprintf(
		"DROP DATABASE IF EXISTS `%s`; CREATE DATABASE `%s`;",
		cfg.Database, cfg.Database,
	)

	args := append([]string{"docker"}, dockerExecArgs(cfg)...)
	args = append(args, mysqlBaseArgs(cfg)...)
	args = append(args, "-e", sql)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("drop/create database failed: %w\noutput: %s", err, string(out))
	}
	return nil
}

// BuildStreamRestoreCmd builds docker exec that streams SQL into mysql on stdin.
// The caller must set cmd.Stdin (e.g. io.MultiReader with preamble, dump, epilogue).
func BuildStreamRestoreCmd(ctx context.Context, cfg Config) (*exec.Cmd, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	args := append([]string{"docker"}, dockerExecArgs(cfg)...)
	args = append(args, mysqlBaseArgs(cfg)...)
	args = append(args, cfg.Database)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	return cmd, nil
}
