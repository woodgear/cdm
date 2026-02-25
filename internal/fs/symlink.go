// Package fs provides filesystem operations for symlink management
package fs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodgear/cdm/pkg/types"
)

// SymlinkManager handles symlink operations
type SymlinkManager struct {
	verbose bool
}

// NewSymlinkManager creates a new symlink manager
func NewSymlinkManager(verbose bool) *SymlinkManager {
	return &SymlinkManager{verbose: verbose}
}

// IsSymlink checks if path is a symlink
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

// ReadSymlink reads the target of a symlink
func ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// IsCorrectSymlink checks if target already points to source
func IsCorrectSymlink(target, source string) bool {
	isLink, err := IsSymlink(target)
	if err != nil || !isLink {
		return false
	}

	currentSource, err := ReadSymlink(target)
	if err != nil {
		return false
	}

	return currentSource == source
}

// FileExists checks if a file exists (not a symlink)
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// CreateSymlink creates a symlink with backup and sudo support
func (sm *SymlinkManager) CreateSymlink(target, source string, opts types.ApplyOptions) error {
	// Check if already correct
	if IsCorrectSymlink(target, source) {
		if sm.verbose {
			fmt.Printf("[SKIP] Already linked: %s -> %s\n", target, source)
		}
		return nil
	}

	// Backup existing file if requested
	if opts.Backup && FileExists(target) {
		isLink, _ := IsSymlink(target)
		if !isLink {
			backupPath := target + ".backup." + time.Now().Format("20060102_150405")
			if !opts.DryRun {
				if err := copyFile(target, backupPath); err != nil {
					return fmt.Errorf("failed to backup %s: %w", target, err)
				}
				if sm.verbose {
					fmt.Printf("[BACKUP] %s -> %s\n", target, backupPath)
				}
			} else {
				fmt.Printf("[DRY-RUN] Would backup: %s -> %s\n", target, backupPath)
			}
		}
	}

	// Remove existing target
	// Remove existing target (use Lstat to detect broken symlinks too)
	if _, err := os.Lstat(target); err == nil {
		if !opts.DryRun {
			if err := os.Remove(target); err != nil {
				if os.IsPermission(err) {
					// Try with sudo
					if err := removeWithSudo(target); err != nil {
						return fmt.Errorf("failed to remove %s (even with sudo): %w", target, err)
					}
				} else {
					return fmt.Errorf("failed to remove %s: %w", target, err)
				}
			}
			if sm.verbose {
				fmt.Printf("[REMOVE] %s\n", target)
			}
		} else {
			fmt.Printf("[DRY-RUN] Would remove: %s\n", target)
		}
	}

	// Create parent directory
	targetDir := filepath.Dir(target)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if !opts.DryRun {
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				if os.IsPermission(err) {
					// Try with sudo
					if err := mkdirWithSudo(targetDir); err != nil {
						return fmt.Errorf("failed to create directory %s (even with sudo): %w", targetDir, err)
					}
				} else {
					return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
				}
			}
			if sm.verbose {
				fmt.Printf("[MKDIR] %s\n", targetDir)
			}
		} else {
			fmt.Printf("[DRY-RUN] Would create directory: %s\n", targetDir)
		}
	}

	// Create symlink
	if !opts.DryRun {
		if err := os.Symlink(source, target); err != nil {
			if os.IsPermission(err) {
				// Try with sudo
				if err := symlinkWithSudo(target, source); err != nil {
					return fmt.Errorf("failed to create symlink %s (even with sudo): %w", target, err)
				}
			} else {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}
		}
		if sm.verbose {
			fmt.Printf("[LINK] %s -> %s\n", target, source)
		}
	} else {
		fmt.Printf("[DRY-RUN] Would link: %s -> %s\n", target, source)
	}

	return nil
}

// copyFile copies a file to a new location
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

// removeWithSudo removes a file using sudo (with terminal access)
func removeWithSudo(path string) error {
	cmd := exec.Command("sudo", "rm", "-f", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// symlinkWithSudo creates a symlink using sudo (with terminal access)
func symlinkWithSudo(target, source string) error {
	cmd := exec.Command("sudo", "ln", "-sf", source, target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// mkdirWithSudo creates directory using sudo (with terminal access)
func mkdirWithSudo(path string) error {
	cmd := exec.Command("sudo", "mkdir", "-p", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExpandPath expands ~ to home directory
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

// NeedsSudo checks if a path requires sudo privileges
// Returns true if path is under system directories
func NeedsSudo(path string) bool {
	// System directories that require root
	systemDirs := []string{
		"/etc",
		"/usr",
		"/var",
		"/root",
		"/opt",
	}

	for _, dir := range systemDirs {
		if strings.HasPrefix(path, dir+"/") || path == dir {
			return true
		}
	}
	return false
}

// IsRoot checks if current process is running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// CheckSudoLinks checks if any links require sudo and returns them
func CheckSudoLinks(links []interface{ GetTarget() string }) []string {
	var sudoLinks []string
	for _, link := range links {
		if NeedsSudo(link.GetTarget()) {
			sudoLinks = append(sudoLinks, link.GetTarget())
		}
	}
	return sudoLinks
}
