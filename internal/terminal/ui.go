package terminal

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	bold    = color.New(color.Bold)
	green   = color.New(color.FgGreen, color.Bold)
	red     = color.New(color.FgRed, color.Bold)
	yellow  = color.New(color.FgYellow)
	cyan    = color.New(color.FgCyan)
	faint   = color.New(color.Faint)
	dimStep = color.New(color.Faint)
)

// Step prints a numbered step with spinner-style progress.
type Step struct {
	total   int
	current int
}

// NewProgress creates a step tracker for n total steps.
func NewProgress(total int) *Step {
	return &Step{total: total}
}

// Start prints the "running" line for the current step and returns a done func.
// Call done(nil) on success or done(err) on failure.
func (s *Step) Start(msg string) func(error) {
	s.current++
	prefix := faint.Sprintf("[%d/%d]", s.current, s.total)
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	stop := make(chan struct{})

	go func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				fmt.Printf("\r%s %s %s", prefix, cyan.Sprint(frames[i%len(frames)]), msg)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()

	return func(err error) {
		close(stop)
		time.Sleep(90 * time.Millisecond)
		if err == nil {
			fmt.Printf("\r%s %s %s\n", prefix, green.Sprint("✓"), msg)
		} else {
			fmt.Printf("\r%s %s %s\n", prefix, red.Sprint("✗"), msg)
		}
	}
}

// Success prints the final success banner.
func Success(appName, simulator, mcpURL string) {
	fmt.Println()
	bold.Printf("Done. %s is running on %s.\n", appName, simulator)
	fmt.Printf("MCP server: %s\n", cyan.Sprint(mcpURL))
	fmt.Println()
}

// InitSuccess prints success for apus init.
func InitSuccess(projectName string) {
	fmt.Println()
	bold.Println("Done.")
	fmt.Printf("Build and run %s, then connect at %s\n",
		projectName, cyan.Sprint("http://localhost:9847/mcp"))
	fmt.Println()
}

// RemoveSuccess prints success for apus remove.
func RemoveSuccess(projectName string) {
	fmt.Println()
	bold.Println("Done.")
	fmt.Printf("Apus has been removed from %s.\n", projectName)
	fmt.Println()
}

// Fatal prints an error and hints, does NOT call os.Exit (let caller decide).
func Fatal(msg string, err error) {
	fmt.Println()
	red.Printf("Error: %s\n", msg)
	if err != nil {
		faint.Printf("  %v\n", err)
	}
}

// Info prints a dim informational line.
func Info(msg string) {
	faint.Println("  " + msg)
}

// Header prints a bold header with blank lines around it.
func Header(msg string) {
	fmt.Println()
	bold.Println(msg)
}

// Detected prints the "Detected: …" summary used by apus init.
func Detected(project, target string) {
	pad := strings.Repeat(" ", 8-len("Target:"))
	fmt.Printf("Detected: %s\n", bold.Sprint(project))
	fmt.Printf("Target:%s%s\n", pad, bold.Sprint(target))
	fmt.Println()
}
