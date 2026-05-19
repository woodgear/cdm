// Package check provides functionality to verify task status
package check

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/woodgear/cdm/internal/fs"
	"github.com/woodgear/cdm/pkg/types"
)

// Checker verifies the status of tasks against a plan
type Checker struct {
	verbose bool
}

// NewChecker creates a new checker
func NewChecker(verbose bool) *Checker {
	return &Checker{verbose: verbose}
}

// CheckPlan verifies all tasks in a plan against the current environment
func (c *Checker) CheckPlan(plan *types.Plan) *types.CheckReport {
	report := &types.CheckReport{
		Total:    len(plan.Tasks),
		ByStatus: make(map[types.TaskStatus]int),
		Results:  make([]types.CheckResult, 0, len(plan.Tasks)),
		AllOK:    true,
	}

	for _, task := range plan.Tasks {
		result := c.checkTask(task)
		report.Results = append(report.Results, result)
		report.ByStatus[result.Status]++

		if result.Status != types.StatusOK {
			report.AllOK = false
		}
	}

	return report
}

// checkTask checks a single task and returns its status
func (c *Checker) checkTask(task types.Task) types.CheckResult {
	result := types.CheckResult{
		Task: task,
	}

	// Check if source exists
	if _, err := os.Stat(task.Source); err != nil {
		result.Status = types.StatusSourceMissing
		if os.IsNotExist(err) {
			result.Detail = fmt.Sprintf("source file does not exist: %s", task.Source)
		} else {
			result.Detail = fmt.Sprintf("failed to stat source %s: %v", task.Source, err)
		}
		return result
	}

	switch task.Action {
	case types.ActionLink:
		return c.checkLinkTask(task, result)
	case types.ActionCopyIfNotExist, types.ActionCopy:
		return c.checkCopyTask(task, result)
	default:
		result.Status = types.StatusMissing
		result.Detail = fmt.Sprintf("unknown task action: %s", task.Action)
		return result
	}
}

func (c *Checker) checkLinkTask(task types.Task, result types.CheckResult) types.CheckResult {
	// Check if target exists
	info, err := os.Lstat(task.Target)
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
	actualSource, err := os.Readlink(task.Target)
	if err != nil {
		result.Status = types.StatusWrongLink
		result.Detail = fmt.Sprintf("failed to read symlink: %v", err)
		return result
	}

	if actualSource == task.Source {
		result.Status = types.StatusOK
		result.Detail = "correctly linked"
	} else {
		result.Status = types.StatusWrongLink
		result.Detail = fmt.Sprintf("points to: %s", actualSource)
	}

	return result
}

func (c *Checker) checkCopyTask(task types.Task, result types.CheckResult) types.CheckResult {
	info, err := os.Lstat(task.Target)
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
	if info.Mode()&os.ModeSymlink != 0 {
		result.Status = types.StatusUnexpectedLink
		result.Detail = "copy target exists as symlink"
		return result
	}

	if task.Action == types.ActionCopy {
		match, err := fs.FileContentsMatch(task.Source, task.Target)
		if err != nil {
			result.Status = types.StatusMismatch
			result.Detail = fmt.Sprintf("failed to compare: %v", err)
			return result
		}
		if !match {
			result.Status = types.StatusMismatch
			result.Detail = "content differs from source"
			return result
		}
		result.Status = types.StatusOK
		result.Detail = "content matches"
		return result
	}

	result.Status = types.StatusOK
	result.Detail = "copy target exists"
	return result
}

// PrintReport prints a formatted check report (Unix style)
func PrintReport(report *types.CheckReport, verbose bool, ignoreOK bool) {
	// Status labels
	labels := map[types.TaskStatus]string{
		types.StatusOK:             "OK",
		types.StatusMissing:        "MISSING",
		types.StatusWrongLink:      "WRONG_LINK",
		types.StatusNotSymlink:     "NOT_SYMLINK",
		types.StatusUnexpectedLink: "UNEXPECTED_LINK",
		types.StatusSourceMissing:  "SOURCE_MISSING",
		types.StatusMismatch:       "MISMATCH",
	}

	// Print results to stdout
	for _, result := range report.Results {
		if ignoreOK && result.Status == types.StatusOK {
			continue
		}
		label := labels[result.Status]
		source := result.Task.Source
		target := result.Task.Target
		fmt.Printf("%s\t%s\t%s\t%s\n", label, result.Task.Action, source, target)
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
