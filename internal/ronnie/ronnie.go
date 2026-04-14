package ronnie

import (
	"fmt"
	"time"
)

func pick(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	i := time.Now().UnixNano() % int64(len(opts))
	if i < 0 {
		i = -i
	}
	return opts[i]
}

// Plain logs (full line for stderr / stdout).
func PlainChecking() string {
	return "db-lifter: " + pick([]string{
		"Yeah buddy — checking container...",
		"Ain't nothin' but a peanut — verifying container...",
	})
}

func PlainRecreate() string {
	return "db-lifter: " + pick([]string{
		"Light weight baby — dropping & recreating database...",
		"Heavy-ass dump, light work — recreating database...",
	})
}

func PlainStreaming() string {
	return "db-lifter: " + pick([]string{
		"Yeah buddy — streaming SQL into MySQL...",
		"Time to lift this dump — streaming SQL...",
	})
}

func PlainCompleted(elapsed time.Duration) string {
	t := elapsed.Truncate(time.Millisecond).String()
	return "db-lifter: " + pick([]string{
		fmt.Sprintf("Yeah buddy — completed in %s", t),
		fmt.Sprintf("Light weight baby — done in %s", t),
	})
}

// TUI captions (chosen once per phase change, not on every redraw).
func TUICaptionInit() string {
	return pick([]string{
		"Yeah buddy — checking container...",
		"Ain't nothin' but a peanut — verifying container...",
	})
}

func TUICaptionDrop() string {
	return pick([]string{
		"Light weight baby — dropping & recreating database...",
		"Heavy-ass schema — we still lifting — recreating database...",
	})
}

func TUICaptionStream() string {
	return pick([]string{
		"Yeah buddy — streaming dump...",
		"Light weight — streaming dump...",
	})
}

func TUIDone() string {
	return pick([]string{
		"\u2714 Yeah buddy — restore complete!",
		"\u2714 Light weight baby — that's the lift!",
	})
}
