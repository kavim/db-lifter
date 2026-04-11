package restore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinmacielmedeiros/db-lift/internal/docker"
	"github.com/kevinmacielmedeiros/db-lift/internal/progress"
)

const (
	streamPreamble = "SET FOREIGN_KEY_CHECKS=0; SET UNIQUE_CHECKS=0; SET AUTOCOMMIT=0;\n"
	streamEpilogue = "\nCOMMIT; SET FOREIGN_KEY_CHECKS=1; SET UNIQUE_CHECKS=1;\n"
)

const maxStderrInError = 64 << 10

type Phase int

const (
	PhaseInit Phase = iota
	PhaseDrop
	PhaseStream
	PhaseDone
	PhaseError
)

type Status struct {
	Phase    Phase
	Progress *progress.Reader
	Err      error
}

type Params struct {
	DumpPath         string
	Container        string
	User             string
	Password         string
	Database         string
	RecreateDatabase bool
}

// Run executes the full restore pipeline, sending Status updates to the channel.
// It blocks until completion or context cancellation.
func Run(ctx context.Context, p Params, statusCh chan<- Status) {
	cfg := docker.Config{
		Container: p.Container,
		User:      p.User,
		Password:  p.Password,
		Database:  p.Database,
	}

	send := func(s Status) {
		select {
		case statusCh <- s:
		case <-ctx.Done():
		}
	}

	send(Status{Phase: PhaseInit})

	if err := docker.CheckContainerRunning(ctx, p.Container); err != nil {
		send(Status{Phase: PhaseError, Err: err})
		return
	}

	if p.RecreateDatabase {
		send(Status{Phase: PhaseDrop})
		if err := docker.DropAndCreateDB(ctx, cfg); err != nil {
			send(Status{Phase: PhaseError, Err: err})
			return
		}
	}

	f, err := os.Open(p.DumpPath)
	if err != nil {
		send(Status{Phase: PhaseError, Err: fmt.Errorf("cannot open dump file: %w", err)})
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		send(Status{Phase: PhaseError, Err: fmt.Errorf("cannot stat dump file: %w", err)})
		return
	}

	total := fi.Size()
	if !fi.Mode().IsRegular() {
		total = -1
	}

	pr := progress.NewReader(f, total)
	stdin := io.MultiReader(strings.NewReader(streamPreamble), pr, strings.NewReader(streamEpilogue))

	send(Status{Phase: PhaseStream, Progress: pr})

	cmd, err := docker.BuildStreamRestoreCmd(ctx, cfg)
	if err != nil {
		send(Status{Phase: PhaseError, Err: err})
		return
	}
	cmd.Stdin = stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		send(Status{Phase: PhaseError, Err: fmt.Errorf("stderr pipe: %w", err)})
		return
	}

	if err := cmd.Start(); err != nil {
		send(Status{Phase: PhaseError, Err: fmt.Errorf("failed to start restore: %w", err)})
		return
	}

	stderrOut, readErr := readStderrLimited(stderr, maxStderrInError)
	if readErr != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		send(Status{Phase: PhaseError, Err: fmt.Errorf("read mysql stderr: %w", readErr)})
		return
	}

	if err := cmd.Wait(); err != nil {
		send(Status{Phase: PhaseError, Err: fmt.Errorf("restore failed: %w\n%s", err, string(stderrOut))})
		return
	}

	send(Status{Phase: PhaseDone})
}

func readStderrLimited(r io.Reader, max int) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(r, int64(max))); err != nil {
		return buf.Bytes(), err
	}
	_, err := io.Copy(io.Discard, r)
	return buf.Bytes(), err
}
