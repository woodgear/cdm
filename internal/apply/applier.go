// Package apply handles plan execution
package apply

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/woodgear/cdm/internal/fs"
	"github.com/woodgear/cdm/pkg/types"
)

// Applier executes deployment plans
type Applier struct {
	verbose bool
	sm      *fs.SymlinkManager
}

// NewApplier creates a new plan applier
func NewApplier(verbose bool) *Applier {
	return &Applier{
		verbose: verbose,
		sm:      fs.NewSymlinkManager(verbose),
	}
}

// ReadPlan reads a plan from a JSON file
func ReadPlan(planFile string) (*types.Plan, error) {
	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan types.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan file: %w", err)
	}

	return &plan, nil
}

// WritePlan writes a plan to a JSON file
func WritePlan(planFile string, plan *types.Plan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(planFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	return nil
}

// Apply executes a plan
func (a *Applier) Apply(plan *types.Plan, opts types.ApplyOptions) error {
	fmt.Printf("[INFO] Applying execution plan...\n")

	if opts.DryRun {
		fmt.Printf("[WARN] DRY-RUN MODE: No changes will be made\n")
	}

	var count, success, skipped int

	for _, link := range plan.Links {
		count++

		if a.verbose {
			fmt.Printf("[%d] %s <- %s (%s)\n", count, link.Target, link.Source, link.Reason)
		}

		// Check if source exists
		if _, err := os.Stat(link.Source); os.IsNotExist(err) {
			fmt.Printf("[WARN] Source file not found, skipping: %s\n", link.Source)
			skipped++
			continue
		}

		// Create symlink
		if err := a.sm.CreateSymlink(link.Target, link.Source, opts); err != nil {
			fmt.Printf("[ERROR] Failed to create symlink: %s\n", err)
			skipped++
			continue
		}

		success++
	}

	fmt.Printf("[SUCCESS] Apply completed\n")
	fmt.Printf("  Total: %d\n", count)
	fmt.Printf("  Success: %d\n", success)
	fmt.Printf("  Skipped: %d\n", skipped)

	return nil
}

// ApplyFromFile reads and applies a plan from a file
func (a *Applier) ApplyFromFile(planFile string, opts types.ApplyOptions) error {
	plan, err := ReadPlan(planFile)
	if err != nil {
		return err
	}

	return a.Apply(plan, opts)
}
