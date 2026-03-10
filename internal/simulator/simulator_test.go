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

func TestBoot_AlreadyBootedIsSuccess(t *testing.T) {
	origState := deviceStateFn
	origRun := runSimctlFn
	t.Cleanup(func() { deviceStateFn = origState; runSimctlFn = origRun })

	// deviceState returns "Shutting Down" — Boot will attempt simctl boot
	deviceStateFn = func(udid string) (string, error) {
		return "Shutting Down", nil
	}
	// simctl boot fails with "already Booted" race condition error
	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		return "An error was encountered processing the command (domain=com.apple.CoreSimulator.SimError, code=405):\nUnable to boot device in current state: Booted", errors.New("exit status 149")
	}

	err := Boot("TEST-UDID")
	if err != nil {
		t.Fatalf("Boot() should succeed when device is already Booted, got: %v", err)
	}
}

func TestBoot_SkipsWhenAlreadyBooted(t *testing.T) {
	origState := deviceStateFn
	origRun := runSimctlFn
	t.Cleanup(func() { deviceStateFn = origState; runSimctlFn = origRun })

	deviceStateFn = func(udid string) (string, error) {
		return "Booted", nil
	}
	bootCalled := false
	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		bootCalled = true
		return "", nil
	}

	err := Boot("TEST-UDID")
	if err != nil {
		t.Fatalf("Boot() should succeed, got: %v", err)
	}
	if bootCalled {
		t.Fatalf("Boot() should not call simctl boot when already Booted")
	}
}

func TestBoot_RealBootError(t *testing.T) {
	origState := deviceStateFn
	origRun := runSimctlFn
	t.Cleanup(func() { deviceStateFn = origState; runSimctlFn = origRun })

	deviceStateFn = func(udid string) (string, error) {
		return "Shutdown", nil
	}
	runSimctlFn = func(timeout time.Duration, env map[string]string, args ...string) (string, error) {
		return "Unable to boot device in current state: Creating", errors.New("exit status 149")
	}

	err := Boot("TEST-UDID")
	if err == nil {
		t.Fatalf("Boot() should fail for non-Booted state errors")
	}
	if !strings.Contains(err.Error(), "simctl boot") {
		t.Fatalf("expected simctl boot error, got: %v", err)
	}
}

func TestWaitForMCPReady_SucceedsOnJSONRPCResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	}))
	t.Cleanup(server.Close)

	if err := waitForMCPReady(server.URL+"/mcp", 500*time.Millisecond, 25*time.Millisecond); err != nil {
		t.Fatalf("expected readiness to succeed, got: %v", err)
	}
}

func TestWaitForMCPReady_RetriesUntilServerHealthy(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not mcp"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"initialize first"}}`))
	}))
	t.Cleanup(server.Close)

	if err := waitForMCPReady(server.URL, 750*time.Millisecond, 20*time.Millisecond); err != nil {
		t.Fatalf("expected readiness to eventually succeed, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Fatalf("expected retries before success, got %d calls", got)
	}
}

func TestWaitForMCPReady_FailsOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>wrong endpoint</html>"))
	}))
	t.Cleanup(server.Close)

	err := waitForMCPReady(server.URL, 150*time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("expected readiness failure message, got: %v", err)
	}
}
