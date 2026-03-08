package simulator

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type simctlCall struct {
	timeout time.Duration
	env     map[string]string
	args    []string
}

func TestLaunchWithProjectRoot_UsesTerminateAndEnv(t *testing.T) {
	orig := runSimctlFn
	t.Cleanup(func() { runSimctlFn = orig })

	var calls []simctlCall
	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		calls = append(calls, simctlCall{timeout: timeout, env: env, args: args})
		return "com.dev.myapp: 123", nil
	}

	err := LaunchWithProjectRoot("SIM-UDID", "com.dev.myapp", "/tmp/MyApp")
	if err != nil {
		t.Fatalf("expected launch to succeed, got: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 simctl call, got %d", len(calls))
	}

	call := calls[0]
	if call.timeout != simctlLaunchTimeout {
		t.Fatalf("expected timeout %s, got %s", simctlLaunchTimeout, call.timeout)
	}
	wantArgs := []string{"launch", "--terminate-running-process", "SIM-UDID", "com.dev.myapp"}
	if strings.Join(call.args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected args: %v", call.args)
	}
	if got := call.env["SIMCTL_CHILD_APUS_PROJECT_ROOT"]; got != "/tmp/MyApp" {
		t.Fatalf("expected SIMCTL_CHILD_APUS_PROJECT_ROOT=/tmp/MyApp, got %q", got)
	}
}

func TestLaunchWithProjectRoot_FallsBackWithoutTerminateFlag(t *testing.T) {
	orig := runSimctlFn
	t.Cleanup(func() { runSimctlFn = orig })

	var calls []simctlCall
	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		calls = append(calls, simctlCall{timeout: timeout, env: env, args: args})
		if len(calls) == 1 {
			return "unknown option --terminate-running-process", errors.New("exit status 64")
		}
		return "com.dev.myapp: 456", nil
	}

	err := LaunchWithProjectRoot("SIM-UDID", "com.dev.myapp", "")
	if err != nil {
		t.Fatalf("expected fallback launch to succeed, got: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 simctl calls, got %d", len(calls))
	}

	firstArgs := strings.Join(calls[0].args, " ")
	secondArgs := strings.Join(calls[1].args, " ")
	if firstArgs != "launch --terminate-running-process SIM-UDID com.dev.myapp" {
		t.Fatalf("unexpected first args: %v", calls[0].args)
	}
	if secondArgs != "launch SIM-UDID com.dev.myapp" {
		t.Fatalf("unexpected fallback args: %v", calls[1].args)
	}
}

func TestLaunchWithProjectRoot_ReturnsErrorWhenBothAttemptsFail(t *testing.T) {
	orig := runSimctlFn
	t.Cleanup(func() { runSimctlFn = orig })

	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		return "simctl failed", errors.New("exit status 1")
	}

	err := LaunchWithProjectRoot("SIM-UDID", "com.dev.myapp", "")
	if err == nil {
		t.Fatalf("expected launch to fail")
	}
	if !strings.Contains(err.Error(), "simctl launch") {
		t.Fatalf("expected launch error, got: %v", err)
	}
}

func TestLooksLikeNotInstalled(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "not installed", output: "App is not installed", want: true},
		{name: "not found", output: "Bundle id not found", want: true},
		{name: "found nothing to uninstall", output: "Found nothing to uninstall", want: true},
		{name: "real error", output: "Failed to create promise", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikeNotInstalled(tc.output)
			if got != tc.want {
				t.Fatalf("looksLikeNotInstalled(%q)=%v want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestWaitForHTTPReady_SucceedsOnHealthyEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if err := waitForHTTPReady(server.URL, 500*time.Millisecond, 25*time.Millisecond); err != nil {
		t.Fatalf("expected readiness to succeed, got: %v", err)
	}
}

func TestWaitForHTTPReady_RetriesUntilServerHealthy(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if err := waitForHTTPReady(server.URL, 750*time.Millisecond, 20*time.Millisecond); err != nil {
		t.Fatalf("expected readiness to eventually succeed, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Fatalf("expected retries before success, got %d calls", got)
	}
}

func TestWaitForHTTPReady_FailsOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	err := waitForHTTPReady(server.URL, 150*time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("expected readiness failure message, got: %v", err)
	}
}
