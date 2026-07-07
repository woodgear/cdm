# CDM - 配置/Dotfile 管理器

轻量级 CLI 工具，通过 link/copy 任务管理 dotfile 和配置文件，支持多层覆盖。

## 特性

- **多层覆盖**: 支持 share（低优先级）和主机特定（高优先级）配置覆盖
- **自动发现**: 根据 hostname 自动发现配置目录
- **文件夹级 Link**: 可配置整个文件夹作为单个 symlink，而非逐个文件
- **Copy Base**: 支持只在目标不存在时复制 seed config，适合会被工具本地改写的配置
- **启动迁移**: 自动备份并迁移旧 `.cdm.conf.json` 元配置到当前 schema
- **子目录配置**: 配置文件可放在任意子目录，灵活管理
- **状态检查**: 检查当前环境与配置的一致性
- **Dry-run 模式**: 应用前预览变更
- **备份支持**: 覆盖前可选备份现有文件
- **Sudo 集成**: 自动处理需要 root 权限的系统目录
- **JSON Plan**: 生成可审查的执行计划

## 安装

### 直接安装

```bash
go install github.com/woodgear/cdm/cmd/cdm@latest
```

如果需要绕过 Go proxy 缓存，直接从 GitHub 拉取最新提交：

```bash
GOPROXY=direct go install github.com/woodgear/cdm/cmd/cdm@latest
```

安装后确保 `$GOPATH/bin` 在 `PATH` 中，然后确认版本：

```bash
cdm version
```

### 本地源码安装

```bash
go install ./cmd/cdm
```

`cdm version` 会自动用 Go build metadata 推导日期版本，优先使用 git commit 时间，其次解析 `go install ...@latest` 的 pseudo-version 或日期 tag。取不到构建来源日期时会显示 `unknown`，不会用运行当天日期伪造版本。

## 快速开始

### 1. 设置配置目录结构

```
$CDM_BASE/
├── share/                    # 通用配置（低优先级）
│   ├── home/                 # 链接到 $HOME 的文件
│   │   ├── .bashrc
│   │   ├── .zshrc
│   │   └── .config/
│   │       └── starship.toml
│   ├── root/                 # 链接到 / 的文件（需要 sudo）
│   │   └── etc/
│   │       └── hosts
└── <hostname>/               # 主机特定配置（高优先级）
    ├── home/
    │   └── .zshrc           # 覆盖 share/home/.zshrc
    └── root/
```

### 2. 设置环境变量

```bash
export CDM_BASE=/path/to/your/configs
```

### 3. 生成计划

```bash
# 自动发现（使用 $CDM_BASE/share 和 $CDM_BASE/<hostname>）
cdm plan

# 或显式指定路径
cdm plan /path/to/share /path/to/hostname
```

### 4. 应用计划

```bash
# 应用并备份
cdm apply --backup

# 或一步完成
cdm deploy --backup
```

## 命令

### `cdm plan [paths...]`

生成执行计划。

```bash
# 自动发现（使用 $CDM_BASE）
cdm plan

# 指定路径
cdm plan ./configs/share ./configs/myhost

# 自定义输出文件
cdm plan -o my-plan.json

# 详细输出
cdm plan -v
```

### `cdm apply [plan-file]`

应用执行计划，执行 link/copy 任务。

```bash
# 应用默认计划文件 (./cdm-plan.json)
cdm apply

# 应用指定计划
cdm apply my-plan.json

# Dry-run（仅显示将执行的操作）
cdm apply -d

# 覆盖前备份
cdm apply --backup

# 详细输出
cdm apply -v
```

### `cdm deploy [paths...]`

一步完成计划生成和应用。

```bash
cdm deploy --backup -v
```

### `cdm check [paths...]`

检查链接状态，验证配置是否正确应用。默认只输出需要处理的项目。

```bash
# 自动发现
cdm check

# 显示 OK 项
cdm check --show-ok

# 指定路径
cdm check /path/to/configs

# 退出码：
#   0 - 所有链接正常
#   1 - 有链接需要处理
```

### `cdm version`

打印版本号。

## 选项

| Flag | Short | 说明 |
|------|-------|------|
| `--verbose` | `-v` | 详细输出 |
| `--dry-run` | `-d` | 仅显示将执行的操作，不实际执行 |
| `--backup` | `-b` | 覆盖前备份现有文件 |
| `--cdm-base` | | 配置基础目录（覆盖 CDM_BASE 环境变量） |
| `--output` | `-o` | 输出计划文件（默认：./cdm-plan.json） |

## 配置

### 目录结构

CDM 期望源目录包含 `home/` 和/或 `root/` 子目录：

```
source/
├── home/          → 链接到 $HOME 的文件
│   ├── .bashrc
│   └── .config/
│       └── starship.toml
└── root/          → 链接到 / 的文件
    └── etc/
        └── hosts
```

### 覆盖优先级

当提供多个源路径时，后面的覆盖前面的：

```bash
cdm plan ./share ./myhost
```

- `./share/home/.zshrc` → 链接到 `~/.zshrc`
- `./myhost/home/.zshrc` → **覆盖**并链接到 `~/.zshrc`

### 自动发现

如果未指定路径且设置了 `CDM_BASE`：

1. `$CDM_BASE/share`（通用配置，低优先级）
2. `$CDM_BASE/<hostname>`（主机特定配置，高优先级）

### 配置文件 (`.cdm.conf.json`)

放在源目录或子目录中，自定义行为：

```json
{
  "version": "2026.05.19",
  "remaps": [
    {
      "source": ".config/Code - OSS/User",
      "target": "~/Library/Application Support/Code/User"
    }
  ],
  "externalLinks": [
    {
      "source": "~/sm/pv/maid/.claude/skills/slock",
      "target": "~/.agents/skills/slock"
    }
  ],
  "copyIfNotExist": [
    {
      "source": "home/.codex/config.toml",
      "target": "~/.codex/config.toml"
    }
  ],
  "linkFolders": [
    "home/.config/nvim",
    "home/.config/zed"
  ],
  "exclude": [
    "*.bak",
    "*.tmp"
  ],
  "hooks": {
    "preApply": "echo '开始部署'",
    "postApply": "echo '部署完成'"
  }
}
```

#### linkFolders - 文件夹级 Link

声明整个文件夹作为单个 symlink，而不是递归链接每个文件：

```json
{
  "linkFolders": ["home/.config/nvim"]
}
```

**效果对比：**

| 不使用 linkFolders | 使用 linkFolders |
|-------------------|-----------------|
| `~/.config/nvim/init.lua` → 单独链接 | `~/.config/nvim` → 整个文件夹链接 |
| `~/.config/nvim/lua/config.lua` → 单独链接 | (通过文件夹链接自动可用) |
| ...更多文件更多链接 | 只有 1 个链接 |

**配置位置：**
- 放在源目录根目录：`linkFolders` 路径相对于根目录
- 放在子目录：`linkFolders` 路径相对于该子目录

#### remaps - 默认 link 目标重映射

将 `home/` 或 `root/` 扫描出来的默认目标重映射到另一个位置：

```json
{
  "remaps": [
    {
      "source": ".config/Code - OSS/User",
      "target": "~/Library/Application Support/Code/User"
    }
  ]
}
```

#### externalLinks - 外部路径 link

把 CDM 管理目录外的文件或目录链接到目标位置。source 不存在时会直接失败。

```json
{
  "externalLinks": [
    {
      "source": "~/sm/pv/maid/.claude/skills/slock",
      "target": "~/.agents/skills/slock"
    }
  ]
}
```

#### copyIfNotExist - Copy Base

目标不存在时复制 source；目标已存在时保持本机文件不变。目标是 symlink 时会报错，避免 copy base 继续退化成 link base。copy source 会从默认 link 扫描中排除，所以它可以放在普通的 `home/` 或 `root/` 树下。

```json
{
  "copyIfNotExist": [
    {
      "source": "home/.codex/config.toml",
      "target": "~/.codex/config.toml"
    }
  ]
}
```

#### copy - 强制复制

每次 apply 都复制 source 到 target。配合 `--backup` 会先备份被覆盖目标。

```json
{
  "copy": [
    {
      "source": "root/etc/example.conf",
      "target": "/etc/example.conf"
    }
  ]
}
```

#### 自动迁移

CDM 加载配置前会扫描当前 source paths 下的 `.cdm.conf.json`。如果发现旧字段，会先生成同目录备份：

```text
.cdm.conf.json.backup.20260519_114530.151090088
```

然后自动迁移：

| 旧字段 | 新字段 |
|--------|--------|
| `pathMappings` 且 source 是 `~` 或绝对路径 | `externalLinks` |
| `pathMappings` 且 source 是相对路径 | `remaps` |
| `configIfNotExist` | `copyIfNotExist` |
| `fileMappings` | `copyIfNotExist` |

迁移后配置使用严格 JSON 解析，未知字段会直接报错。

#### exclude - 排除文件

排除特定模式的文件：

```json
{
  "exclude": ["*.bak", "*.tmp", "*.swp"]
}
```

#### hooks - 钩子

在应用前后执行命令：

```json
{
  "hooks": {
    "preApply": "echo 'Starting deployment'",
    "postApply": "echo 'Deployment complete'"
  }
}
```

## Plan 文件格式

生成的计划是 JSON 文件：

```json
{
  "version": "2026.05.19",
  "timestamp": "2026-02-25T23:57:43+08:00",
  "hostname": "myhost",
  "sources": ["/path/to/share", "/path/to/myhost"],
  "tasks": [
    {
      "source": "/path/to/share/home/.zshrc",
      "target": "/home/user/.zshrc",
      "action": "link",
      "reason": "new"
    }
  ],
  "stats": {
    "total": 44,
    "link": 41,
    "copyIfNotExist": 0,
    "copy": 0,
    "override": 3,
    "skip": 0
  }
}
```

## Sudo 支持

CDM 自动检测需要提升权限的操作（如 `/etc`、`/usr` 下的文件），并在需要时提示输入 sudo 密码。

## License

MIT
