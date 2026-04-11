package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/mattn/go-isatty"
	"github.com/kevinmacielmedeiros/db-lift/internal/docker"
	"github.com/kevinmacielmedeiros/db-lift/internal/progress"
	"github.com/kevinmacielmedeiros/db-lift/internal/restore"
	"github.com/kevinmacielmedeiros/db-lift/internal/tui"
	"github.com/spf13/cobra"
)

// version is overridden at link time via -ldflags "-X main.version=..."
var version = "dev"

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func main() {
	var (
		params         restore.Params
		envFile        string
		noTUI          bool
		timeout        time.Duration
	)

	rootCmd := &cobra.Command{
		Use:   "db-lift",
		Short: "High-performance MySQL Docker restore CLI",
		Long:  "Restores large MySQL dump files into Docker containers using streaming and an optional TUI.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if envFile != "" {
				if err := godotenv.Load(envFile); err != nil {
					return &exitError{2, fmt.Errorf("failed to load env file %q: %w", envFile, err)}
				}
			}

			fillFromEnv(&params.DumpPath, "DB_LIFT_FILE")
			fillFromEnv(&params.Container, "DB_LIFT_CONTAINER")
			fillFromEnv(&params.User, "DB_LIFT_USER")
			fillFromEnv(&params.Password, "DB_LIFT_PASSWORD")
			fillFromEnv(&params.Database, "DB_LIFT_DATABASE")

			if params.User == "" {
				params.User = "root"
			}

			if !cmd.Flags().Lookup("recreate-database").Changed {
				if v := os.Getenv("DB_LIFT_RECREATE"); v == "1" || strings.EqualFold(v, "true") {
					params.RecreateDatabase = true
				}
			}
			if !cmd.Flags().Lookup("no-tui").Changed {
				if v := os.Getenv("DB_LIFT_NO_TUI"); v == "1" || strings.EqualFold(v, "true") {
					noTUI = true
				}
			}

			if params.DumpPath == "" {
				return &exitError{2, fmt.Errorf("--file (-f) or DB_LIFT_FILE is required")}
			}
			if params.Container == "" {
				return &exitError{2, fmt.Errorf("--container (-c) or DB_LIFT_CONTAINER is required")}
			}
			if params.Database == "" {
				return &exitError{2, fmt.Errorf("--database (-d) or DB_LIFT_DATABASE is required")}
			}
			if err := docker.ValidateConfig(docker.Config{
				User:     params.User,
				Database: params.Database,
			}); err != nil {
				return &exitError{2, fmt.Errorf("invalid configuration: %w", err)}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return execute(params, noTUI, timeout)
		},
		Version: version,
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	flags := rootCmd.Flags()
	flags.StringVarP(&envFile, "env", "e", "", "Load configuration from an env file (e.g. --env .env)")
	flags.StringVarP(&params.DumpPath, "file", "f", "", "Path to the SQL dump file")
	flags.StringVarP(&params.Container, "container", "c", "", "Docker container name or ID")
	flags.StringVarP(&params.User, "user", "u", "", "MySQL user (default \"root\")")
	flags.StringVarP(&params.Password, "password", "p", "", "MySQL password (prefer env DB_LIFT_PASSWORD)")
	flags.StringVarP(&params.Database, "database", "d", "", "Target database name")
	flags.BoolVar(&params.RecreateDatabase, "recreate-database", false, "Drop and recreate the database before restore (destructive)")
	flags.BoolVar(&noTUI, "no-tui", false, "Plain log output (also implied when stdout is not a TTY or CI is set)")
	flags.DurationVar(&timeout, "timeout", 0, "Maximum duration for the whole operation (0 = no limit)")

	if err := rootCmd.Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			fmt.Fprintln(os.Stderr, ee.Error())
			os.Exit(ee.code)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func execute(params restore.Params, noTUI bool, timeout time.Duration) error {
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-ctx.Done():
		}
	}()

	statusCh := make(chan restore.Status, 8)
	go func() {
		defer close(statusCh)
		restore.Run(ctx, params, statusCh)
	}()

	usePlain := noTUI || os.Getenv("CI") != "" || !isatty.IsTerminal(os.Stdout.Fd())
	start := time.Now()

	if usePlain {
		return runPlain(ctx, statusCh, start)
	}

	p := tea.NewProgram(tui.NewModel(statusCh, cancel), tea.WithAltScreen())
	raw, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	m := raw.(tui.Model)
	final := m.FinalStatus()

	switch final.Phase {
	case restore.PhaseDone:
		fmt.Printf("\nRestore completed successfully in %s\n", m.Elapsed())
		return nil
	case restore.PhaseError:
		fmt.Fprintf(os.Stderr, "\nRestore failed: %v\n", final.Err)
		return final.Err
	default:
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		return fmt.Errorf("interrupted")
	}
}

func runPlain(ctx context.Context, statusCh <-chan restore.Status, start time.Time) error {
	var progressCancel context.CancelFunc
	defer func() {
		if progressCancel != nil {
			progressCancel()
		}
	}()

	for s := range statusCh {
		switch s.Phase {
		case restore.PhaseInit:
			fmt.Fprintln(os.Stderr, "db-lift: checking container...")
		case restore.PhaseDrop:
			fmt.Fprintln(os.Stderr, "db-lift: dropping and recreating database...")
		case restore.PhaseStream:
			fmt.Fprintln(os.Stderr, "db-lift: streaming SQL into MySQL...")
			if s.Progress != nil {
				pctx, c := context.WithCancel(context.Background())
				progressCancel = c
				go plainProgressLoop(pctx, s.Progress)
			}
		case restore.PhaseDone:
			if progressCancel != nil {
				progressCancel()
			}
			fmt.Fprintf(os.Stdout, "db-lift: completed in %s\n", time.Since(start).Truncate(time.Millisecond))
			return nil
		case restore.PhaseError:
			if progressCancel != nil {
				progressCancel()
			}
			fmt.Fprintf(os.Stderr, "db-lift: %v\n", s.Err)
			return s.Err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("interrupted")
}

func plainProgressLoop(ctx context.Context, pr *progress.Reader) {
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr)
			return
		case <-t.C:
			if pr.Indeterminate() {
				fmt.Fprintf(os.Stderr, "\rdb-lift: streamed %s   ", formatBytes(pr.BytesRead()))
				continue
			}
			fmt.Fprintf(os.Stderr, "\rdb-lift: %.1f%%  %s / %s   ",
				pr.Percent()*100, formatBytes(pr.BytesRead()), formatBytes(pr.Total()))
		}
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// fillFromEnv sets *target from the environment variable if *target is still empty.
func fillFromEnv(target *string, envKey string) {
	if *target == "" {
		if v, ok := os.LookupEnv(envKey); ok {
			*target = v
		}
	}
}
