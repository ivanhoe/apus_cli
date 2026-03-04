// Package simulator wraps xcrun simctl operations.
package simulator

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const preferredDeviceName = "iPhone 16e"

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
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "--json").Output()
	if err != nil {
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
	state, err := deviceState(udid)
	if err != nil {
		return err
	}
	if state == "Booted" {
		return nil
	}

	if err := exec.Command("xcrun", "simctl", "boot", udid).Run(); err != nil {
		return fmt.Errorf("simctl boot: %w", err)
	}

	// Wait until booted (up to 60s)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if s, _ := deviceState(udid); s == "Booted" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("simulator did not boot within 60 seconds")
}

// Install installs an app bundle on the simulator.
func Install(udid, appPath string) error {
	cmd := exec.Command("xcrun", "simctl", "install", udid, appPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("simctl install: %w\n%s", err, string(out))
	}
	return nil
}

// Launch launches an installed app and returns the PID.
func Launch(udid, bundleID string) error {
	cmd := exec.Command("xcrun", "simctl", "launch", udid, bundleID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("simctl launch: %w\n%s", err, string(out))
	}
	return nil
}

// OpenSimulatorApp opens the Simulator.app UI so the user can see it.
func OpenSimulatorApp() error {
	return exec.Command("open", "-a", "Simulator").Run()
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
