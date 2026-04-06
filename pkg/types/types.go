// Package types defines the core data structures for CDM
package types

import "time"

// Config represents the .cdm.conf.json configuration file structure
type Config struct {
	Version       string        `json:"version,omitempty"`
	PathMappings  []PathMapping `json:"pathMappings,omitempty"`
	FileMappings  []PathMapping `json:"fileMappings,omitempty"` // Files to copy (not symlink) for consistency
	Exclude       []string      `json:"exclude,omitempty"`
	LinkFolders   []string      `json:"linkFolders,omitempty"`  // Directories to link as a whole (relative to this config's location)
	Hooks         *Hooks        `json:"hooks,omitempty"`
	Repos         []RepoConfig  `json:"repos,omitempty"`        // Git repositories to manage
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
	Path   string `json:"path"`            // Relative path from config file location
	URL    string `json:"url"`             // Clone URL (required)
	Branch string `json:"branch"`          // Target branch (required)
	Remote string `json:"remote,omitempty"` // Remote name (default: origin)
}

// Plan represents the execution plan structure
type Plan struct {
	Version   string       `json:"version"`
	Timestamp time.Time    `json:"timestamp"`
	Hostname  string       `json:"hostname"`
	Sources   []string     `json:"sources"`
	Links     []Link       `json:"links"`
	Repos     []RepoConfig `json:"repos,omitempty"`
	Stats     Stats        `json:"stats"`
}

// Link represents a single deployment operation (symlink or copy)
type Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Action string `json:"action"` // "link" | "copy"
	Reason string `json:"reason"` // "new" | "override from <name>" | "file mapping"
}

// Stats contains execution statistics
type Stats struct {
	Total    int `json:"total"`
	New      int `json:"new"`
	Override int `json:"override"`
	Skip     int `json:"skip"`
}

// FileEntry represents a file discovered during scanning
type FileEntry struct {
	Source     string // Absolute source path
	Target     string // Absolute target path
	SourcePath string // Source directory this file belongs to
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

// LinkStatus represents the status of a link check
type LinkStatus string

const (
	StatusOK           LinkStatus = "OK"            // Symlink/copy exists and is correct
	StatusMissing      LinkStatus = "MISSING"       // Target does not exist
	StatusWrongLink    LinkStatus = "WRONG_LINK"   // Target is symlink but points to wrong source
	StatusNotSymlink   LinkStatus = "NOT_SYMLINK"  // Target exists but is not a symlink
	StatusSourceMissing LinkStatus = "SOURCE_MISSING" // Source file does not exist
	StatusMismatch     LinkStatus = "MISMATCH"     // Copy target content differs from source
)

// CheckResult represents the result of checking a single link
 type CheckResult struct {
	Link   Link
	Status LinkStatus
	Detail string // Additional detail (e.g., actual link target if wrong)
}

// CheckReport represents the full check report
 type CheckReport struct {
	Total    int
	ByStatus map[LinkStatus]int
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
