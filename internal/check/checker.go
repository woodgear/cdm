// Package check provides functionality to verify symlink status
package check

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/woodgear/cdm/pkg/types"
)

// Checker verifies the status of symlinks against a plan
type Checker struct {
	verbose bool
}

// NewChecker creates a new checker
func NewChecker(verbose bool) *Checker {
	return &Checker{verbose: verbose}
}

// CheckPlan verifies all links in a plan against the current environment
func (c *Checker) CheckPlan(plan *types.Plan) *types.CheckReport {
	report := &types.CheckReport{
		Total:    len(plan.Links),
		ByStatus: make(map[types.LinkStatus]int),
		Results:  make([]types.CheckResult, 0, len(plan.Links)),
		AllOK:    true,
	}

	for _, link := range plan.Links {
		result := c.checkLink(link)
		report.Results = append(report.Results, result)
		report.ByStatus[result.Status]++

		if result.Status != types.StatusOK {
			report.AllOK = false
		}
	}

	return report
}

// checkLink checks a single link and returns its status
func (c *Checker) checkLink(link types.Link) types.CheckResult {
	result := types.CheckResult{
		Link: link,
	}

	// Check if source exists
	if _, err := os.Stat(link.Source); os.IsNotExist(err) {
		result.Status = types.StatusSourceMissing
		result.Detail = fmt.Sprintf("source file does not exist: %s", link.Source)
		return result
	}

	// Check if target exists
	info, err := os.Lstat(link.Target)
	if os.IsNotExist(err) {
		result.Status = types.StatusMissing
		result.Detail = "target does not exist"
		return result
	}
	if err != nil {
		result.Status = types.StatusMissing
		result.Detail = fmt.Sprintf("failed to stat target: %v", err)
		return result
	}

	// Check if target is a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		result.Status = types.StatusNotSymlink
		result.Detail = "target exists but is not a symlink"
		return result
	}

	// Check if symlink points to correct source
	actualSource, err := os.Readlink(link.Target)
	if err != nil {
		result.Status = types.StatusWrongLink
		result.Detail = fmt.Sprintf("failed to read symlink: %v", err)
		return result
	}

	if actualSource == link.Source {
		result.Status = types.StatusOK
		result.Detail = "correctly linked"
	} else {
		result.Status = types.StatusWrongLink
		result.Detail = fmt.Sprintf("points to: %s", actualSource)
	}

	return result
}

// PrintReport prints a formatted check report (Unix style)
func PrintReport(report *types.CheckReport, verbose bool) {
	// Status labels
	labels := map[types.LinkStatus]string{
		types.StatusOK:           "OK",
		types.StatusMissing:      "MISSING",
		types.StatusWrongLink:    "WRONG_LINK",
		types.StatusNotSymlink:   "NOT_SYMLINK",
		types.StatusSourceMissing: "SOURCE_MISSING",
	}

	// Print results to stdout
	for _, result := range report.Results {
		label := labels[result.Status]
		target := result.Link.Target
		source := result.Link.Source
		fmt.Printf("%s\t%s\t%s\n", label, target, source)
	}
}

// CheckFromFile reads a plan file and checks it
func (c *Checker) CheckFromFile(planFile string) (*types.CheckReport, error) {
	plan, err := readPlanFile(planFile)
	if err != nil {
		return nil, err
	}

	return c.CheckPlan(plan), nil
}

// readPlanFile reads a plan from a JSON file
func readPlanFile(planFile string) (*types.Plan, error) {
	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan types.Plan
	if err := parsePlan(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan file: %w", err)
	}

	return &plan, nil
}

// parsePlan parses JSON data into a Plan struct
func parsePlan(data []byte, plan *types.Plan) error {
	return json.Unmarshal(data, plan)
}
