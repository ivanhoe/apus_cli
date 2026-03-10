// Package simulator wraps xcrun simctl operations.
package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const preferredDeviceName = "iPhone 16e"

const (
	simctlListTimeout    = 15 * time.Second
	simctlBootTimeout    = 20 * time.Second
	simctlShutdownDelay  = 20 * time.Second
	simctlUninstallDelay = 15 * time.Second
	simctlInstallTimeout = 90 * time.Second
	simctlLaunchTimeout  = 20 * time.Second
	mcpPollInterval      = 300 * time.Millisecond
)

var runSimctlFn = runSimctl
var deviceStateFn = deviceState

// Device represents a simulator device.
type Device struct {
	UDID         string `json:"udid"`
	Name         string `json:"name"`
	State        string `json:"state"`
	IsAvailable  bool   `json:"isAvailable"`
	DeviceTypeID string `json:"deviceTypeIdentifier"`
}

// ListAvailable returns all available simulator devices.
func ListAvailable() ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), simctlListTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "available", "--json")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("simctl list timed out after %s", simctlListTimeout)
		}
		return nil, fmt.Errorf("simctl list: %w", err)
	}

	var result struct {
		Devices map[string][]Device `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse simctl output: %w", err)
	}

	var all []Device
	for _, devices := range result.Devices {
		all = append(all, devices...)
	}
	return all, nil
}

// PickBestDevice returns the preferred device (iPhone 16e first, then newest iPhone).
func PickBestDevice() (Device, error) {
	devices, err := ListAvailable()
	if err != nil {
		return Device{}, err
	}

	// Prefer iPhone 16e
	for _, d := range devices {
		if d.IsAvailable && strings.Contains(d.Name, preferredDeviceName) {
			return d, nil
		}
	}

	// Fall back to any available iPhone
	for _, d := range devices {
		if d.IsAvailable && strings.Contains(d.Name, "iPhone") {
			return d, nil
		}
	}

	return Device{}, fmt.Errorf("no available iPhone simulator found — open Xcode > Settings > Platforms to download one")
}

// Boot boots the simulator if not already booted.
func Boot(udid string) error {
	state, err := deviceStateFn(udid)
	if err != nil {
		return err
	}
	if state == "Booted" {
		return nil
	}

	if out, err := runSimctlFn(simctlBootTimeout, nil, "boot", udid); err != nil {
		// Handle race: state changed to Booted between our check and the boot call
		if strings.Contains(out, "current state: Booted") {
			return nil
		}
		return fmt.Errorf("simctl boot: %w\n%s", err, out)
	}

	// Wait until booted (up to 60s)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if s, _ := deviceStateFn(udid); s == "Booted" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("simulator did not boot within 60 seconds")
}

// Shutdown stops a simulator if it's currently booted.
func Shutdown(udid string) error {
	state, err := deviceStateFn(udid)
	if err != nil {
		return err
	}
	if state == "Shutdown" {
		return nil
	}

	out, err := runSimctlFn(simctlShutdownDelay, nil, "shutdown", udid)
	if err != nil {
		return fmt.Errorf("simctl shutdown: %w\n%s", err, out)
	}
	return nil
}

// ShutdownOtherBootedDevices stops every booted simulator except the target one.
// This avoids localhost MCP collisions when multiple simulators run apps that bind the same port.
func ShutdownOtherBootedDevices(targetUDID string) error {
	devices, err := ListAvailable()
	if err != nil {
		return err
	}

	var failures []string
	for _, d := range devices {
		if d.State != "Booted" || d.UDID == targetUDID {
			continue
		}
		if out, shutdownErr := runSimctlFn(simctlShutdownDelay, nil, "shutdown", d.UDID); shutdownErr != nil {
			failures = append(failures, fmt.Sprintf("%s (%s): %v %s", d.Name, d.UDID, shutdownErr, strings.TrimSpace(out)))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("failed shutting down other simulators: %s", strings.Join(failures, "; "))
	}
	return nil
}

// Install installs an app bundle on the simulator.
func Install(udid, appPath string) error {
	out, err := runSimctlFn(simctlInstallTimeout, nil, "install", udid, appPath)
	if err != nil {
		return fmt.Errorf("simctl install: %w\n%s", err, out)
	}
	return nil
}

// UninstallIfPresent removes an app if it is already installed.
// "Not installed" errors are ignored.
func UninstallIfPresent(udid, bundleID string) error {
	out, err := runSimctlFn(simctlUninstallDelay, nil, "uninstall", udid, bundleID)
	if err != nil && !looksLikeNotInstalled(out) {
		return fmt.Errorf("simctl uninstall: %w\n%s", err, out)
	}
	return nil
}

// Launch launches an installed app and returns the PID.
func Launch(udid, bundleID string) error {
	return LaunchWithProjectRoot(udid, bundleID, "")
}

// LaunchWithProjectRoot launches an installed app, force-restarting an existing process
// and optionally injecting APUS_PROJECT_ROOT into the app process for hot-reload lookup.
func LaunchWithProjectRoot(udid, bundleID, projectRoot string) error {
	env := map[string]string{}
	if strings.TrimSpace(projectRoot) != "" {
		env["SIMCTL_CHILD_APUS_PROJECT_ROOT"] = projectRoot
	}

	args := []string{"launch", "--terminate-running-process", udid, bundleID}
	out, err := runSimctlFn(simctlLaunchTimeout, env, args...)
	if err == nil {
		return nil
	}

	// Older simctl versions may not support --terminate-running-process.
	fallbackArgs := []string{"launch", udid, bundleID}
	fallbackOut, fallbackErr := runSimctlFn(simctlLaunchTimeout, env, fallbackArgs...)
	if fallbackErr != nil {
		return fmt.Errorf("simctl launch: %w\n%s", err, out)
	}
	_ = fallbackOut
	return nil
}

// OpenSimulatorApp opens the Simulator.app UI so the user can see it.
func OpenSimulatorApp() error {
	return exec.Command("open", "-a", "Simulator").Run()
}

// WaitForMCPReady polls the given URL until the server responds or timeout elapses.
func WaitForMCPReady(url string, timeout time.Duration) error {
	return waitForHTTPReady(url, timeout, mcpPollInterval)
}

func runSimctl(timeout time.Duration, env map[string]string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := append([]string{"simctl"}, args...)
	cmd := exec.CommandContext(ctx, "xcrun", cmdArgs...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("timed out after %s", timeout)
	}
	return string(out), err
}

func looksLikeNotInstalled(output string) bool {
	text := strings.ToLower(output)
	return strings.Contains(text, "not installed") ||
		strings.Contains(text, "not found") ||
		strings.Contains(text, "found nothing to uninstall")
}

func deviceState(udid string) (string, error) {
	devices, err := ListAvailable()
	if err != nil {
		return "", err
	}
	for _, d := range devices {
		if d.UDID == udid {
			return d.State, nil
		}
	}
	return "", fmt.Errorf("device %s not found", udid)
}

func waitForHTTPReady(url string, timeout, interval time.Duration) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("URL is empty")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be > 0")
	}
	if interval <= 0 {
		interval = mcpPollInterval
	}

	client := &http.Client{Timeout: 1200 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	var lastFailure error

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			// Drain body to allow connection reuse; errors are harmless in a poll loop.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode < 500 {
				return nil
			}
			lastFailure = fmt.Errorf("received HTTP %d", resp.StatusCode)
		} else {
			lastFailure = err
		}

		time.Sleep(interval)
	}

	if lastFailure != nil {
		return fmt.Errorf("endpoint %s not ready after %s: %v", url, timeout, lastFailure)
	}
	return fmt.Errorf("endpoint %s not ready after %s", url, timeout)
}
