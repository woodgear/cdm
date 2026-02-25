// Package types defines the core data structures for CDM
package types

import "time"

// Config represents the .cdm.conf.json configuration file structure
type Config struct {
	Version      string        `json:"version,omitempty"`
	PathMappings []PathMapping `json:"pathMappings,omitempty"`
	Exclude      []string      `json:"exclude,omitempty"`
	Hooks        *Hooks        `json:"hooks,omitempty"`
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

// Plan represents the execution plan structure
type Plan struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Hostname  string    `json:"hostname"`
	Sources   []string  `json:"sources"`
	Links     []Link    `json:"links"`
	Stats     Stats     `json:"stats"`
}

// Link represents a single symlink operation
type Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Action string `json:"action"` // "link"
	Reason string `json:"reason"` // "new" | "override from <name>"
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
