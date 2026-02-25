# CDM - Config/Dotfile Manager

A lightweight CLI tool for managing dotfiles and configuration files with multi-layer override support. CDM creates symlinks from source configuration files to target locations on your system.

## Features

- **Multi-layer Override**: Support for shared (low priority) and host-specific (high priority) configurations
- **Auto-discovery**: Automatically discovers config directories based on hostname
- **Dry-run Mode**: Preview changes before applying
- **Backup Support**: Optionally backup existing files before overwriting
- **Sudo Integration**: Handles system directories requiring elevated privileges
- **JSON Plan**: Generates executable plans for review before deployment

## Installation

```bash
# Build from source
cd cdm
go build -o cdm ./cmd/cdm

# Or with version info
go build -ldflags "-X main.version=$(git describe --tags)" -o cdm ./cmd/cdm
```

## Quick Start

### 1. Set up your config directory structure

```
$CDM_BASE/
├── share/                    # Common config (low priority)
│   ├── home/                 # Files to link to $HOME
│   │   ├── .bashrc
│   │   ├── .zshrc
│   │   └── .config/
│   │       └── starship.toml
│   └── root/                 # Files to link to / (requires sudo)
│       └── etc/
│           └── hosts
└── <hostname>/               # Host-specific config (high priority)
    ├── home/
    │   └── .zshrc           # Overrides share/home/.zshrc
    └── root/
```

### 2. Set environment variable

```bash
export CDM_BASE=/path/to/your/configs
```

### 3. Generate a plan (dry-run by default)

```bash
# Auto-discover from $CDM_BASE/share and $CDM_BASE/<hostname>
cdm plan -d

# Or specify paths explicitly
cdm plan /path/to/share /path/to/hostname -d
```

### 4. Apply the plan

```bash
# Apply with backup
cdm apply --backup

# Or deploy in one step
cdm deploy --backup
```

## Commands

### `cdm plan [paths...]`

Generate an execution plan from source directories.

```bash
# Auto-discover (uses $CDM_BASE)
cdm plan

# Specify paths
cdm plan ./configs/share ./configs/myhost

# Custom output file
cdm plan -o my-plan.json

# Verbose output
cdm plan -v
```

### `cdm apply [plan-file]`

Apply an execution plan to create symlinks.

```bash
# Apply default plan file (./cdm-plan.json)
cdm apply

# Apply specific plan
cdm apply my-plan.json

# Dry-run (show what would be done)
cdm apply -d

# Backup existing files before overwriting
cdm apply --backup

# Verbose output
cdm apply -v
```

### `cdm deploy [paths...]`

Generate and apply a plan in one step.

```bash
cdm deploy --backup -v
```

### `cdm version`

Print the version number.

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Verbose output |
| `--dry-run` | `-d` | Show what would be done without executing |
| `--backup` | `-b` | Backup existing files before overwriting |
| `--cdm-base` | | Base configuration directory (overrides CDM_BASE env var) |
| `--output` | `-o` | Output plan file (default: ./cdm-plan.json) |

## Configuration

### Directory Structure

CDM expects source directories to contain `home/` and/or `root/` subdirectories:

```
source/
├── home/          → Files to link to $HOME
│   ├── .bashrc
│   └── .config/
│       └── starship.toml
└── root/          → Files to link to /
    └── etc/
        └── hosts
```

### Override Priority

When multiple source paths are provided, later paths override earlier ones:

```bash
cdm plan ./share ./myhost
```

- `./share/home/.zshrc` → links to `~/.zshrc`
- `./myhost/home/.zshrc` → **overrides** and links to `~/.zshrc`

### Auto-discovery

If no paths are specified and `CDM_BASE` is set:

1. `$CDM_BASE/share` (common config, low priority)
2. `$CDM_BASE/<hostname>` (host-specific config, high priority)

### Optional Config File (`.cdm.conf.json`)

Place in source directories to customize behavior:

```json
{
  "version": "1.0.0",
  "pathMappings": [
    {
      "source": ".config/nvim",
      "target": "~/.config/nvim"
    }
  ],
  "exclude": [
    "*.bak",
    "*.tmp"
  ],
  "hooks": {
    "preApply": "echo 'Starting deployment'",
    "postApply": "echo 'Deployment complete'"
  }
}
```

## Plan File Format

Generated plans are JSON files:

```json
{
  "version": "1.0.0",
  "timestamp": "2026-02-25T23:57:43+08:00",
  "hostname": "myhost",
  "sources": ["/path/to/share", "/path/to/myhost"],
  "links": [
    {
      "source": "/path/to/share/home/.zshrc",
      "target": "/home/user/.zshrc",
      "action": "link",
      "reason": "new"
    }
  ],
  "stats": {
    "total": 44,
    "new": 41,
    "override": 3,
    "skip": 0
  }
}
```

## Sudo Support

CDM automatically detects when operations require elevated privileges (e.g., files under `/etc`, `/usr`) and will prompt for sudo when needed.

## License

MIT
