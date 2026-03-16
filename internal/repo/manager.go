// Package repo provides git repository management functionality
package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/woodgear/cdm/pkg/types"
)

// Manager handles git repository operations
type Manager struct {
	verbose bool
}

// NewManager creates a new repo manager
func NewManager(verbose bool) *Manager {
	return &Manager{verbose: verbose}
}

// IsGitRepo checks if path is a git repository
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode()&os.ModeSymlink != 0
}

// GetRemoteURL gets the URL of a remote
func GetRemoteURL(path, remote string) (string, error) {
	cmd := exec.Command("git", "-C", path, "remote", "get-url", remote)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranch gets the current branch name
func GetCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Clone clones a repository
func (m *Manager) Clone(url, path string) error {
	if m.verbose {
		fmt.Printf("[CLONE] %s -> %s\n", url, path)
	}
	cmd := exec.Command("git", "clone", url, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckoutBranch checks out a branch, creating it if necessary
func (m *Manager) CheckoutBranch(path, branch string) error {
	if m.verbose {
		fmt.Printf("[CHECKOUT] %s: %s\n", path, branch)
	}
	// Try checkout first, then create if fails
	cmd := exec.Command("git", "-C", path, "checkout", branch)
	if err := cmd.Run(); err != nil {
		// Try creating the branch
		cmd = exec.Command("git", "-C", path, "checkout", "-b", branch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

// Pull pulls the latest changes from remote
func (m *Manager) Pull(path, remote, branch string) error {
	if m.verbose {
		fmt.Printf("[PULL] %s: %s/%s\n", path, remote, branch)
	}
	cmd := exec.Command("git", "-C", path, "pull", remote, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetSyncStatus gets the ahead/behind count compared to remote
func GetSyncStatus(path, remote, branch string) (ahead, behind int, err error) {
	// Fetch first (silently)
	exec.Command("git", "-C", path, "fetch", remote).Run()

	cmd := exec.Command("git", "-C", path, "rev-list", "--left-right", "--count",
		fmt.Sprintf("%s/%s...HEAD", remote, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get sync status: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &behind)
		fmt.Sscanf(parts[1], "%d", &ahead)
	}
	return ahead, behind, nil
}

// RepoInfo contains information about a discovered repo
type RepoInfo struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Remote string `json:"remote"`
}

// ScanRepos scans a directory for all git repositories
func ScanRepos(rootPath string) ([]RepoInfo, error) {
	var repos []RepoInfo

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories except .git itself
		if strings.Contains(path, "/.") && !strings.HasSuffix(path, "/.git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this is a git repo
		if info.IsDir() && IsGitRepo(path) {
			relPath, _ := filepath.Rel(rootPath, path)
			if relPath == "." {
				relPath = "."
			}

			// Get remote URL (default: origin)
			remote := "origin"
			url, err := GetRemoteURL(path, remote)
			if err != nil {
				// Try to find any remote
				cmd := exec.Command("git", "-C", path, "remote")
				output, rerr := cmd.Output()
				if rerr == nil {
					remotes := strings.Fields(string(output))
					if len(remotes) > 0 {
						remote = remotes[0]
						url, _ = GetRemoteURL(path, remote)
					}
				}
			}

			// Get current branch
			branch, _ := GetCurrentBranch(path)

			repos = append(repos, RepoInfo{
				Path:   relPath,
				URL:    url,
				Branch: branch,
				Remote: remote,
			})

			// Don't walk into .git directory
			return filepath.SkipDir
		}

		return nil
	})

	return repos, err
}

// DeployRepo deploys a repository according to config
func (m *Manager) DeployRepo(absPath string, config types.RepoConfig, dryRun bool) types.RepoCheckResult {
	result := types.RepoCheckResult{
		Config: config,
	}

	remote := config.Remote
	if remote == "" {
		remote = "origin"
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		// Directory doesn't exist - clone it
		result.Status = types.RepoStatusMissing
		if dryRun {
			result.Detail = fmt.Sprintf("would clone %s to %s", config.URL, absPath)
			return result
		}
		if err := m.Clone(config.URL, absPath); err != nil {
			result.Status = types.RepoStatusMissing
			result.Detail = fmt.Sprintf("failed to clone: %v", err)
			return result
		}
		result.Status = types.RepoStatusCloned
		result.Detail = "cloned successfully"
		// Checkout branch
		if err := m.CheckoutBranch(absPath, config.Branch); err != nil {
			result.Detail = fmt.Sprintf("cloned but failed to checkout branch: %v", err)
		}
		return result
	}

	// Directory exists
	if !info.IsDir() {
		result.Status = types.RepoStatusNotRepo
		result.Detail = "path exists but is not a directory"
		return result
	}

	// Check if it's a git repo
	if !IsGitRepo(absPath) {
		result.Status = types.RepoStatusNotRepo
		result.Detail = "directory exists but is not a git repository"
		return result
	}

	// Check remote URL
	currentURL, err := GetRemoteURL(absPath, remote)
	if err != nil || currentURL != config.URL {
		result.Status = types.RepoStatusWrongRepo
		result.Detail = fmt.Sprintf("wrong remote URL: expected %s, got %s", config.URL, currentURL)
		return result
	}

	// Get current branch
	currentBranch, _ := GetCurrentBranch(absPath)
	result.CurrentBranch = currentBranch

	// Checkout branch if needed
	if currentBranch != config.Branch {
		if dryRun {
			result.Status = types.RepoStatusWrongBranch
			result.Detail = fmt.Sprintf("would checkout %s (current: %s)", config.Branch, currentBranch)
			return result
		}
		if err := m.CheckoutBranch(absPath, config.Branch); err != nil {
			result.Status = types.RepoStatusWrongBranch
			result.Detail = fmt.Sprintf("failed to checkout branch: %v", err)
			return result
		}
	}

	// Pull latest
	if !dryRun {
		if err := m.Pull(absPath, remote, config.Branch); err != nil {
			if m.verbose {
				fmt.Printf("[WARN] pull failed: %v\n", err)
			}
		}
	}

	result.Status = types.RepoStatusOK
	result.Detail = "deployed successfully"
	return result
}

// CheckRepo checks the status of a repository
func (m *Manager) CheckRepo(absPath string, config types.RepoConfig) types.RepoCheckResult {
	result := types.RepoCheckResult{
		Config: config,
	}

	remote := config.Remote
	if remote == "" {
		remote = "origin"
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		result.Status = types.RepoStatusMissing
		result.Detail = "directory does not exist"
		return result
	}

	if !info.IsDir() {
		result.Status = types.RepoStatusNotRepo
		result.Detail = "path exists but is not a directory"
		return result
	}

	// Check if it's a git repo
	if !IsGitRepo(absPath) {
		result.Status = types.RepoStatusNotRepo
		result.Detail = "directory exists but is not a git repository"
		return result
	}

	// Check remote URL
	currentURL, err := GetRemoteURL(absPath, remote)
	if err != nil || currentURL != config.URL {
		result.Status = types.RepoStatusWrongRepo
		result.Detail = fmt.Sprintf("wrong remote URL: expected %s, got %s", config.URL, currentURL)
		return result
	}

	// Get current branch
	currentBranch, _ := GetCurrentBranch(absPath)
	result.CurrentBranch = currentBranch

	if currentBranch != config.Branch {
		result.Status = types.RepoStatusWrongBranch
		result.Detail = fmt.Sprintf("wrong branch: expected %s, current %s", config.Branch, currentBranch)
		return result
	}

	// Check sync status
	ahead, behind, _ := GetSyncStatus(absPath, remote, config.Branch)
	result.Ahead = ahead
	result.Behind = behind

	if ahead > 0 || behind > 0 {
		result.Status = types.RepoStatusNotSynced
		result.Detail = fmt.Sprintf("not synced: ahead %d, behind %d", ahead, behind)
		return result
	}

	result.Status = types.RepoStatusOK
	result.Detail = "ok"
	return result
}

// PrintScanResult prints repo scan results as JSON
func PrintScanResult(repos []RepoInfo) error {
	output := struct {
		Repos []RepoInfo `json:"repos"`
	}{Repos: repos}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
