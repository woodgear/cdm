// Package cli provides command-line interface for CDM
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/woodgear/cdm/internal/apply"
	"github.com/woodgear/cdm/internal/check"
	"github.com/woodgear/cdm/internal/plan"
	"github.com/woodgear/cdm/internal/repo"
	"github.com/woodgear/cdm/pkg/types"
)

var (
	// Version is set at build time
	Version   = "1.0.0"
	GitCommit = "unknown"
	GitBranch = "unknown"
	BuildDate = "unknown"

	// Global flags
	flagVerbose bool
	flagDryRun  bool
	flagBackup  bool
	flagCdmBase string
	flagOutput  string

	// Check-specific flags
	flagIgnoreOK bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "cdm",
	Short: "CDM - Config/Dotfile Manager",
	Long: `CDM (Config/Dotfile Manager) is a tool for managing dotfiles
with multi-layer override support. It creates symlinks from source
configuration files to target locations.`,
}

// planCmd represents the plan command
var planCmd = &cobra.Command{
	Use:   "plan [paths...]",
	Short: "Generate execution plan",
	Long: `Generate an execution plan from source directories.

Source directories should contain 'home/' and/or 'root/' subdirectories:
  source/
  ├── home/          → Files to link to $HOME
  │   ├── .bashrc
  │   └── .config/
  └── root/          → Files to link to /
      └── etc/
          └── hosts

If no paths are specified and CDM_BASE is set, paths are auto-discovered:
  - $CDM_BASE/share (common config, low priority)
  - $CDM_BASE/<hostname> (host-specific config, high priority)`,
	RunE: runPlan,
}

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply [plan-file]",
	Short: "Apply execution plan",
	Long: `Apply an execution plan to create symlinks.

If no plan file is specified, uses ./cdm-plan.json by default.`,
	RunE: runApply,
}

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy [paths...]",
	Short: "Plan and apply in one step",
	Long: `Generate and apply an execution plan in one step.

This is equivalent to running 'plan' followed by 'apply'.`,
	RunE: runDeploy,
}

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check [paths...]",
	Short: "Check link status",
	Long: `Check if all links are correctly applied.

If no paths are specified and CDM_BASE is set, paths are auto-discovered:
  - $CDM_BASE/share (common config, low priority)
  - $CDM_BASE/<hostname> (host-specific config, high priority)

Exit codes:
  0 - All links OK
  1 - Some links need attention`,
	RunE: runCheck,
}

// repoScanCmd represents the repo-scan command
var repoScanCmd = &cobra.Command{
	Use:   "repo-scan [path]",
	Short: "Scan directory for git repos",
	Long: `Scan a directory for all git repositories and output as JSON.

Output format can be used directly in .cdm.conf.json repos section.

Example:
  cdm repo-scan ~/projects > repos.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRepoScan,
}
func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Show what would be done without executing")
	rootCmd.PersistentFlags().BoolVarP(&flagBackup, "backup", "b", false, "Backup existing files before overwriting")
	rootCmd.PersistentFlags().StringVar(&flagCdmBase, "cdm-base", "", "Base configuration directory (overrides CDM_BASE env var)")

	// Plan-specific flags
	planCmd.Flags().StringVarP(&flagOutput, "output", "o", "./cdm-plan.json", "Output plan file")

	// Check-specific flags
	checkCmd.Flags().BoolVar(&flagIgnoreOK, "ignore-ok", false, "Hide OK status entries")

	// Add commands
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(repoScanCmd)

	// Completion command
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate shell completion script for cdm.

Load completions:

  Bash:
    cdm completion bash > /etc/bash_completion.d/cdm
    # or per-user:
    cdm completion bash > ~/.local/share/bash-completion/completions/cdm

  Zsh:
    cdm completion zsh > ~/.zfunc/_cdm
    # ensure ~/.zfunc is in fpath before compinit in .zshrc:
    #   fpath+=~/.zfunc

  Fish:
    cdm completion fish > ~/.config/fish/completions/cdm.fish

  PowerShell:
    cdm completion powershell > cdm.ps1
    # and source it in your profile`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell type: %s", args[0])
			}
		},
	}
	rootCmd.AddCommand(completionCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("CDM v%s\n", Version)
			fmt.Printf("  branch: %s\n", GitBranch)
			fmt.Printf("  commit: %s\n", GitCommit)
			fmt.Printf("  built:  %s\n", BuildDate)
		},
	})
}

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}

// getCdmBase returns the CDM base path from flag or environment
func getCdmBase() string {
	if flagCdmBase != "" {
		return flagCdmBase
	}
	return os.Getenv("CDM_BASE")
}

// getAutoDiscoverPaths returns auto-discovered paths based on CDM_BASE
func getAutoDiscoverPaths() ([]string, error) {
	cdmBase := getCdmBase()
	if cdmBase == "" {
		return nil, fmt.Errorf("no source paths specified and CDM_BASE not set")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	sharePath := filepath.Join(cdmBase, "share")
	hostnamePath := filepath.Join(cdmBase, hostname)

	return []string{sharePath, hostnamePath}, nil
}

// getSourcePaths returns source paths from args or auto-discovery
func getSourcePaths(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	paths, err := getAutoDiscoverPaths()
	if err != nil {
		return nil, err
	}

	if flagVerbose {
		fmt.Printf("[INFO] Auto-discovered paths: %v\n", paths)
	}

	return paths, nil
}

func runPlan(cmd *cobra.Command, args []string) error {
	// Get source paths
	sourcePaths, err := getSourcePaths(args)
	if err != nil {
		return err
	}

	// Generate plan
	generator := plan.NewGenerator(flagVerbose)
	p, err := generator.Generate(sourcePaths)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Write plan to file
	if err := apply.WritePlan(flagOutput, p); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	fmt.Printf("[SUCCESS] Plan generated: %s\n", flagOutput)
	fmt.Printf("  Total files: %d\n", p.Stats.Total)
	fmt.Printf("  New: %d\n", p.Stats.New)
	fmt.Printf("  Override: %d\n", p.Stats.Override)

	if flagVerbose {
		fmt.Println("\n[INFO] Plan preview:")
		data, _ := apply.ReadPlan(flagOutput)
		for _, link := range data.Links {
			fmt.Printf("  %s -> %s (%s)\n", link.Target, link.Source, link.Reason)
		}
	}

	return nil
}

func runApply(cmd *cobra.Command, args []string) error {
	planFile := "./cdm-plan.json"
	if len(args) > 0 {
		planFile = args[0]
	}

	// Check if plan file exists
	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		return fmt.Errorf("plan file not found: %s", planFile)
	}

	// Apply plan
	applier := apply.NewApplier(flagVerbose)
	opts := types.ApplyOptions{
		DryRun:  flagDryRun,
		Backup:  flagBackup,
		Verbose: flagVerbose,
	}

	return applier.ApplyFromFile(planFile, opts)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Get source paths
	sourcePaths, err := getSourcePaths(args)
	if err != nil {
		return err
	}

	// Generate temporary plan
	tmpPlan := fmt.Sprintf("/tmp/cdm-deploy-%d.json", os.Getpid())
	defer os.Remove(tmpPlan)

	// Generate plan
	generator := plan.NewGenerator(flagVerbose)
	p, err := generator.Generate(sourcePaths)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Write plan
	if err := apply.WritePlan(tmpPlan, p); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	// Apply plan (symlinks)
	applier := apply.NewApplier(flagVerbose)
	opts := types.ApplyOptions{
		DryRun:  flagDryRun,
		Backup:  flagBackup,
		Verbose: flagVerbose,
	}

	if err := applier.Apply(p, opts); err != nil {
		return err
	}

	// Deploy repos
	if len(p.Repos) > 0 {
		fmt.Printf("\n[INFO] Deploying %d repos...\n", len(p.Repos))
		manager := repo.NewManager(flagVerbose)
		for _, r := range p.Repos {
			result := manager.DeployRepo(r.Path, r, flagDryRun)
			printRepoResult(result)
		}
	}

	return nil
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Get source paths (same pattern as plan/deploy)
	sourcePaths, err := getSourcePaths(args)
	if err != nil {
		return err
	}

	// Generate plan (like deploy)
	generator := plan.NewGenerator(flagVerbose)
	p, err := generator.Generate(sourcePaths)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	allOK := true

	// Check symlinks
	if len(p.Links) > 0 {
		checker := check.NewChecker(flagVerbose)
		report := checker.CheckPlan(p)
		check.PrintReport(report, flagVerbose, flagIgnoreOK)
		if !report.AllOK {
			allOK = false
		}
	}

	// Check repos
	if len(p.Repos) > 0 {
		fmt.Printf("\n[INFO] Checking %d repos...\n", len(p.Repos))
		manager := repo.NewManager(flagVerbose)
		for _, r := range p.Repos {
			result := manager.CheckRepo(r.Path, r)
			printRepoCheckResult(result)
			if result.Status != types.RepoStatusOK {
				allOK = false
			}
		}
	}

	// Return exit code based on result
	if !allOK {
		os.Exit(1)
	}

	return nil
}

func runRepoScan(cmd *cobra.Command, args []string) error {
	scanPath := "."
	if len(args) > 0 {
		scanPath = args[0]
	}

	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	repos, err := repo.ScanRepos(absPath)
	if err != nil {
		return fmt.Errorf("failed to scan repos: %w", err)
	}

	return repo.PrintScanResult(repos)
}

func printRepoResult(result types.RepoCheckResult) {
	statusLabel := string(result.Status)
	switch result.Status {
	case types.RepoStatusOK, types.RepoStatusCloned:
		fmt.Printf("[OK] %s: %s\n", result.Config.Path, result.Detail)
	case types.RepoStatusMissing:
		if result.Detail != "" && len(result.Detail) > 4 && result.Detail[:4] == "would" {
			fmt.Printf("[DRY-RUN] %s: %s\n", result.Config.Path, result.Detail)
		} else {
			fmt.Printf("[CLONE] %s: %s -> %s\n", result.Config.Path, result.Config.URL, result.Config.Branch)
		}
	default:
		fmt.Printf("[%s] %s: %s\n", statusLabel, result.Config.Path, result.Detail)
	}
}

func printRepoCheckResult(result types.RepoCheckResult) {
	fmt.Printf("%s\t%s\t%s", result.Status, result.Config.Path, result.CurrentBranch)
	if result.Ahead > 0 || result.Behind > 0 {
		fmt.Printf("\tahead:%d,behind:%d", result.Ahead, result.Behind)
	}
	if result.Detail != "" && result.Detail != "ok" {
		fmt.Printf("\t%s", result.Detail)
	}
	fmt.Println()
}
