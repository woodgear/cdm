// Package types defines the core data structures for CDM
package types

import "time"

// Config represents the .cdm.conf.json configuration file structure
type Config struct {
	Version        string        `json:"version,omitempty"`
	Remaps         []PathMapping `json:"remaps,omitempty"`
	ExternalLinks  []PathMapping `json:"externalLinks,omitempty"`
	CopyIfNotExist []PathMapping `json:"copyIfNotExist,omitempty"`
	Copy           []PathMapping `json:"copy,omitempty"`
	Exclude        []string      `json:"exclude,omitempty"`
	LinkFolders    []string      `json:"linkFolders,omitempty"` // Directories to link as a whole (relative to this config's location)
	Hooks          *Hooks        `json:"hooks,omitempty"`
	Repos          []RepoConfig  `json:"repos,omitempty"` // Git repositories to manage
}

// PathMapping defines a source-to-target path mapping rule
type PathMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Hooks defines commands to run before and after applying
type Hooks struct {
	PreApply  string `json:"preApply,omitempty"`
	PostApply string `json:"postApply,omitempty"`
}

// RepoConfig represents a git repository configuration
type RepoConfig struct {
	Path   string `json:"path"`             // Relative path from config file location
	URL    string `json:"url"`              // Clone URL (required)
	Branch string `json:"branch"`           // Target branch (required)
	Remote string `json:"remote,omitempty"` // Remote name (default: origin)
}

// Plan represents the execution plan structure
type Plan struct {
	Version   string       `json:"version"`
	Timestamp time.Time    `json:"timestamp"`
	Hostname  string       `json:"hostname"`
	Sources   []string     `json:"sources"`
	Tasks     []Task       `json:"tasks"`
	Repos     []RepoConfig `json:"repos,omitempty"`
	Stats     Stats        `json:"stats"`
}

const (
	ActionLink           = "link"
	ActionCopyIfNotExist = "copy_if_not_exist"
	ActionCopy           = "copy"
)

// Task represents a single filesystem operation.
type Task struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Action string `json:"action"`
	Reason string `json:"reason"` // "new" | "override from <name>"
}

// Stats contains execution statistics
type Stats struct {
	Total          int `json:"total"`
	Link           int `json:"link"`
	CopyIfNotExist int `json:"copyIfNotExist"`
	Copy           int `json:"copy"`
	Override       int `json:"override"`
	Skip           int `json:"skip"`
}

// FileEntry represents a file discovered during scanning
type FileEntry struct {
	Source     string // Absolute source path
	Target     string // Absolute target path
	SourcePath string // Source directory this file belongs to
	Action     string // Action to perform
	Reason     string // Reason for inclusion
}

// GlobalOptions holds global CLI options
type GlobalOptions struct {
	Verbose bool
	DryRun  bool
	Backup  bool
	CdmBase string
}

// ApplyOptions holds options for the apply operation
type ApplyOptions struct {
	DryRun  bool
	Backup  bool
	Verbose bool
}

// TaskStatus represents the status of a task check
type TaskStatus string

const (
	StatusOK             TaskStatus = "OK"
	StatusMissing        TaskStatus = "MISSING"
	StatusWrongLink      TaskStatus = "WRONG_LINK"
	StatusNotSymlink     TaskStatus = "NOT_SYMLINK"
	StatusUnexpectedLink TaskStatus = "UNEXPECTED_LINK"
	StatusSourceMissing  TaskStatus = "SOURCE_MISSING"
	StatusMismatch       TaskStatus = "MISMATCH"
)

// CheckResult represents the result of checking a single task
type CheckResult struct {
	Task   Task
	Status TaskStatus
	Detail string // Additional detail (e.g., actual link target if wrong)
}

// CheckReport represents the full check report
type CheckReport struct {
	Total    int
	ByStatus map[TaskStatus]int
	Results  []CheckResult
	AllOK    bool
}

// RepoStatus represents the status of a repo check
type RepoStatus string

const (
	RepoStatusOK          RepoStatus = "OK"           // Repo exists, correct branch, synced
	RepoStatusMissing     RepoStatus = "MISSING"      // Directory does not exist
	RepoStatusCloned      RepoStatus = "CLONED"       // Just cloned
	RepoStatusWrongBranch RepoStatus = "WRONG_BRANCH" // Wrong branch checked out
	RepoStatusNotSynced   RepoStatus = "NOT_SYNCED"   // Ahead or behind remote
	RepoStatusNotRepo     RepoStatus = "NOT_REPO"     // Exists but not a git repo
	RepoStatusWrongRepo   RepoStatus = "WRONG_REPO"   // Wrong remote URL
)

// RepoCheckResult represents the result of checking a single repo
type RepoCheckResult struct {
	Config        RepoConfig
	Status        RepoStatus
	CurrentBranch string
	Ahead         int
	Behind        int
	Detail        string
}

// RepoCheckReport represents the full repo check report
type RepoCheckReport struct {
	Total    int
	ByStatus map[RepoStatus]int
	Results  []RepoCheckResult
	AllOK    bool
}
