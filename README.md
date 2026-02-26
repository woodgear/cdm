# CDM - 配置/Dotfile 管理器

轻量级 CLI 工具，通过创建符号链接管理 dotfile 和配置文件，支持多层覆盖。

## 特性

- **多层覆盖**: 支持 share（低优先级）和主机特定（高优先级）配置覆盖
- **自动发现**: 根据 hostname 自动发现配置目录
- **文件夹级 Link**: 可配置整个文件夹作为单个 symlink，而非逐个文件
- **子目录配置**: 配置文件可放在任意子目录，灵活管理
- **状态检查**: 检查当前环境与配置的一致性
- **Dry-run 模式**: 应用前预览变更
- **备份支持**: 覆盖前可选备份现有文件
- **Sudo 集成**: 自动处理需要 root 权限的系统目录
- **JSON Plan**: 生成可审查的执行计划

## 安装

```bash
# 从源码构建
cd cdm
go build -o cdm ./cmd/cdm

# 带版本信息
go build -ldflags "-X main.version=$(git describe --tags)" -o cdm ./cmd/cdm
```

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
│   └── root/                 # 链接到 / 的文件（需要 sudo）
│       └── etc/
│           └── hosts
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

应用执行计划，创建符号链接。

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

检查链接状态，验证配置是否正确应用。

```bash
# 自动发现
cdm check

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
  "version": "1.0.0",
  "pathMappings": [
    {
      "source": ".config/nvim",
      "target": "~/.config/nvim"
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

#### pathMappings - 路径映射

将源路径映射到不同的目标路径：

```json
{
  "pathMappings": [
    {
      "source": ".config/nvim",
      "target": "~/.config/nvim"
    }
  ]
}
```

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

## Sudo 支持

CDM 自动检测需要提升权限的操作（如 `/etc`、`/usr` 下的文件），并在需要时提示输入 sudo 密码。

## License

MIT
