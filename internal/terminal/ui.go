package terminal

import (
	"fmt"
	"strings"
	"sync"
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

// Spinner prints an unbounded spinner for long-running work that does not have
// discrete numbered steps.
type Spinner struct {
	msg     string
	mu      sync.RWMutex
	stop    chan struct{}
	stopped chan struct{}
}

// NewProgress creates a step tracker for n total steps.
func NewProgress(total int) *Step {
	return &Step{total: total}
}

// NewSpinner starts a spinner immediately with the given message.
func NewSpinner(msg string) *Spinner {
	s := &Spinner{
		msg:     msg,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	go func() {
		defer close(s.stopped)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.stop:
				return
			default:
				s.mu.RLock()
				msg := s.msg
				s.mu.RUnlock()
				fmt.Printf("\r%s %s", cyan.Sprint(frames[i%len(frames)]), msg)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()

	return s
}

// Update changes the message rendered by the spinner.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop stops the spinner and leaves the last message in the terminal output.
func (s *Spinner) Stop() {
	close(s.stop)
	<-s.stopped
	s.mu.RLock()
	msg := s.msg
	s.mu.RUnlock()
	fmt.Printf("\r%s %s\n", faint.Sprint("·"), msg)
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

// StatusIntegrated prints the integration status when Apus is present.
func StatusIntegrated(pbxproj, swift, agents bool) {
	check := green.Sprint("✓")
	cross := faint.Sprint("–")
	fmt.Println("Apus integration:")
	if pbxproj {
		fmt.Printf("  %s SPM dependency in project.pbxproj\n", check)
	} else {
		fmt.Printf("  %s SPM dependency in project.pbxproj\n", cross)
	}
	if swift {
		fmt.Printf("  %s import Apus + Apus.shared.start() in Swift code\n", check)
	} else {
		fmt.Printf("  %s import Apus + Apus.shared.start() in Swift code\n", cross)
	}
	if agents {
		fmt.Printf("  %s AGENTS.md\n", check)
	} else {
		fmt.Printf("  %s AGENTS.md\n", cross)
	}
	fmt.Println()
}

// StatusNotIntegrated prints the status when Apus is not present.
func StatusNotIntegrated() {
	fmt.Printf("Apus is %s in this project.\n", bold.Sprint("not integrated"))
	Info("Run `apus init` to add it.")
	fmt.Println()
}

// DryRunHeader prints the dry-run banner.
func DryRunHeader() {
	yellow.Println("Dry run — no files will be modified.")
	fmt.Println()
}

// DryRunItem prints a single dry-run action.
func DryRunItem(action, file string) {
	fmt.Printf("  %s %s\n", yellow.Sprint(action), file)
}

// Summary prints a post-execution summary of changed files.
func Summary(changes []FileChange) {
	if len(changes) == 0 {
		return
	}
	fmt.Println()
	faint.Println("Changes:")
	for _, c := range changes {
		faint.Printf("  %s %s (%s)\n", c.Action, c.File, c.Detail)
	}
}

// FileChange describes a single file modification for the summary.
type FileChange struct {
	Action string // "modified", "created", "deleted"
	File   string // relative path
	Detail string // e.g. "+15 lines"
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
