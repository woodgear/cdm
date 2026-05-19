// Package fs provides filesystem operations for symlink management
package fs

import (
	"fmt"
	"io"
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

// isDirWritable checks if the directory containing the target path is writable
// by attempting to create a temporary file in that directory
func isDirWritable(target string) bool {
	dir := filepath.Dir(target)
	// Try to create a test file to check write permission
	testFile := filepath.Join(dir, ".cdm-write-test-"+time.Now().Format("20060102150405.000"))
	err := os.WriteFile(testFile, []byte{}, 0644)
	if err != nil {
		return false
	}
	os.Remove(testFile)
	return true
}

// CopyIfNotExist copies source to target only when target is absent.
func (sm *SymlinkManager) CopyIfNotExist(target, source string, opts types.ApplyOptions) error {
	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("target exists as symlink: %s", target)
		}
		if sm.verbose {
			fmt.Printf("[SKIP] Already exists: %s\n", target)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat %s: %w", target, err)
	}

	return sm.CopyPath(target, source, false, opts)
}

// CopyPath copies a file or directory to target.
func (sm *SymlinkManager) CopyPath(target, source string, overwrite bool, opts types.ApplyOptions) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", source, err)
	}

	targetDir := filepath.Dir(target)
	if _, err := os.Stat(targetDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat target directory %s: %w", targetDir, err)
		}
		if opts.DryRun {
			fmt.Printf("[DRY-RUN] Would create directory: %s\n", targetDir)
		} else if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
		}
	}

	if _, err := os.Lstat(target); err == nil {
		if !overwrite {
			return fmt.Errorf("target already exists: %s", target)
		}
		if opts.Backup {
			backupPath := target + ".backup." + time.Now().Format("20060102_150405")
			if opts.DryRun {
				fmt.Printf("[DRY-RUN] Would backup: %s -> %s\n", target, backupPath)
			} else if err := copyPath(target, backupPath, true); err != nil {
				return fmt.Errorf("failed to backup %s: %w", target, err)
			}
		}
		if opts.DryRun {
			fmt.Printf("[DRY-RUN] Would remove: %s\n", target)
		} else if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("failed to remove %s: %w", target, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat target %s: %w", target, err)
	}

	if opts.DryRun {
		fmt.Printf("[DRY-RUN] Would copy: %s -> %s\n", source, target)
		return nil
	}

	if sourceInfo.IsDir() {
		if err := copyDir(source, target); err != nil {
			return err
		}
	} else if err := copyFile(source, target); err != nil {
		return err
	}
	if sm.verbose {
		fmt.Printf("[COPY] %s -> %s\n", source, target)
	}
	return nil
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

	// Proactively check if we need sudo (directory writeability check)
	// This matches the bash version's: [[ -w "$(dirname "$target")" ]]
	needsSudo := !isDirWritable(target)
	if needsSudo && sm.verbose {
		fmt.Printf("[SUDO] Directory not writable, will use sudo for: %s\n", target)
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

	// Remove existing target (use Lstat to detect broken symlinks too)
	if _, err := os.Lstat(target); err == nil {
		if !opts.DryRun {
			var err error
			if needsSudo {
				// Use sudo proactively when directory is not writable
				err = removeWithSudo(target)
			} else {
				err = os.Remove(target)
			}
			if err != nil {
				return fmt.Errorf("failed to remove %s: %w", target, err)
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
			var err error
			if needsSudo {
				// Use sudo proactively when directory is not writable
				err = mkdirWithSudo(targetDir)
			} else {
				err = os.MkdirAll(targetDir, 0755)
			}
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
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
		var err error
		if needsSudo {
			// Use sudo proactively when directory is not writable
			err = symlinkWithSudo(target, source)
		} else {
			err = os.Symlink(source, target)
		}
		if err != nil {
			return fmt.Errorf("failed to create symlink %s: %w", target, err)
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
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, dstPath); err != nil {
				return err
			}
			continue
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyPath(src, dst string, overwrite bool) error {
	if overwrite {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

// CopyFile copies a source file to target with backup, sudo, and dry-run support
func (sm *SymlinkManager) CopyFile(target, source string, opts types.ApplyOptions) error {
	needsSudo := !isDirWritable(target)
	if needsSudo && sm.verbose {
		fmt.Printf("[SUDO] Directory not writable, will use sudo for: %s\n", target)
	}

	// Backup existing file if requested
	if opts.Backup {
		isLink, _ := IsSymlink(target)
		if !isLink {
			if _, err := os.Lstat(target); err == nil {
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
	}

	// Create parent directory
	targetDir := filepath.Dir(target)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if !opts.DryRun {
			var err error
			if needsSudo {
				err = mkdirWithSudo(targetDir)
			} else {
				err = os.MkdirAll(targetDir, 0755)
			}
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
			}
			if sm.verbose {
				fmt.Printf("[MKDIR] %s\n", targetDir)
			}
		} else {
			fmt.Printf("[DRY-RUN] Would create directory: %s\n", targetDir)
		}
	}

	// Copy file
	if !opts.DryRun {
		var err error
		if needsSudo {
			err = copyWithSudo(target, source)
		} else {
			err = copyFile(source, target)
		}
		if err != nil {
			return fmt.Errorf("failed to copy %s -> %s: %w", source, target, err)
		}
		if sm.verbose {
			fmt.Printf("[COPY] %s -> %s\n", source, target)
		}
	} else {
		fmt.Printf("[DRY-RUN] Would copy: %s -> %s\n", source, target)
	}

	return nil
}

// copyWithSudo copies a file using sudo
func copyWithSudo(target, source string) error {
	cmd := exec.Command("sudo", "cp", source, target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// FileContentsMatch checks if two files have identical content
func FileContentsMatch(a, b string) (bool, error) {
	dataA, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	dataB, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return string(dataA) == string(dataB), nil
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
