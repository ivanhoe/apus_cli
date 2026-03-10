package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ivanhoe/apus_cli/internal/fixturematrix"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("fixturematrix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	manifestPath := fs.String("manifest", filepath.Join("fixtures", "matrix.json"), "Path to the fixture manifest")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	command := "plan"
	if fs.NArg() > 0 {
		command = fs.Arg(0)
	}

	manifest, err := fixturematrix.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	manifestDir := filepath.Dir(*manifestPath)

	switch command {
	case "validate":
		if err := manifest.ValidatePaths(manifestDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Manifest OK: %d fixtures\n", len(manifest.Fixtures))
		return 0
	case "list":
		printList(manifest)
		return 0
	case "plan":
		printPlan(manifest)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (use: validate, list, plan)\n", command)
		return 2
	}
}

func printList(manifest *fixturematrix.Manifest) {
	fixtures := append([]fixturematrix.Fixture(nil), manifest.Fixtures...)
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })

	for _, fixture := range fixtures {
		target := "auto"
		if fixture.TargetRequired {
			target = "required"
		}
		fmt.Printf("%s | %s | %s | %s | target=%s\n",
			fixture.ID,
			fixture.Stage,
			fixture.SourceKind,
			fixture.ExpectedOutcome,
			target,
		)
	}
}

func printPlan(manifest *fixturematrix.Manifest) {
	counts := manifest.CountsByStage()
	fmt.Printf("Fixture matrix v%d\n", manifest.Version)
	fmt.Printf("Ready: %d\n", counts[fixturematrix.StageReady])
	fmt.Printf("Planned: %d\n", counts[fixturematrix.StagePlanned])
	fmt.Println()
	fmt.Println("Next planned fixtures:")

	for _, fixture := range manifest.PlannedFixtures() {
		fmt.Printf("- %s (%s, %s)\n", fixture.ID, fixture.SourceKind, fixture.ExpectedOutcome)
		if fixture.Notes != "" {
			fmt.Printf("  %s\n", fixture.Notes)
		}
	}
}
