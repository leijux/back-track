# BackTrack - 文件备份和还原工具

BackTrack 是一个用 Go 语言编写的高性能文件备份和还原工具，支持多路径备份、文件排除等功能。

## ✨ 功能特性

- **多路径备份**: 支持同时备份多个文件和目录
- **智能排除**: 支持目录名称和文件模式排除规则
- **脚本执行**: 支持备份/还原前后执行自定义脚本
- **高性能**: 并发处理文件，提高备份和还原效率
- **进度显示**: 实时显示备份/还原进度条
- **压缩存储**: 使用最佳压缩算法减少存储空间
- **配置管理**: 支持 YAML 配置文件，易于管理和维护
- **脚本管理**: 支持从配置文件或备份包单独执行脚本

## 🚀 快速开始

### 安装

```bash
# 从源码编译安装
go install github.com/leijux/back-track@latest
```

### 使用方法

```bash
# 备份文件
backtrack backup -c config.yaml -o backup.zip

# 还原文件
backtrack restore -i backup.zip -r /restore/path
```

## 📋 配置文件示例

创建 `config.yaml` 文件：

```yaml
# 备份路径列表（支持文件和目录）
backup_paths:
  - /path/to/dir1      # 备份整个目录
  - /path/to/file1.txt # 备份单个文件

# 排除的目录名称（精确匹配）
exclude_dirs:
  - dir_name           # 排除名为 dir_name 的目录

# 排除的文件模式（支持通配符）
exclude_files:
  - "*.log"            # 排除所有.log文件
  - "*.tmp"            # 排除所有.tmp文件

# 前置脚本（在备份/还原前执行）
before_script: |
  echo "开始备份/还原操作"
  # 可以在这里执行预处理操作，如停止服务、清理临时文件等

# 后置脚本（在备份/还原后执行）
after_script: |
  echo "备份/还原操作完成"
  # 可以在这里执行后处理操作，如启动服务、发送通知等
```

## 🔧 命令行参数

### 全局参数
以下参数适用于所有命令：

```bash
  -q, --quiet     静默模式，不输出日志
```

### backup 命令
```bash
backtrack backup [flags]

Flags:
  -c, --config string    配置文件路径 (默认 "config.yaml")
  -o, --output string    备份输出路径 (默认 "backup_时间戳.zip")
```

### restore 命令
```bash
backtrack restore [flags]

Flags:
  -i, --input string     备份文件路径 (必需)
  -r, --root-dir string  还原根目录 (默认 "/")
  -b, --backup-before-restore   还原前备份，保留最近3个备份
  -s, --script           执行脚本 (默认 true)
```

### script 命令
```bash
backtrack script [flags]

执行备份或还原的前置/后置脚本，支持从YAML配置文件或备份包中读取脚本。

Flags:
  -c, --config string    YAML配置文件路径
  -i, --input string     备份文件路径
  -t, --type string      脚本类型 (before|after) (默认 "before")

示例:
  # 从YAML配置文件执行前置脚本
  backtrack script --type before --config config.yaml
  
  # 从备份包执行后置脚本
  backtrack script --type after --input backup.zip
```

### config 命令
```bash
backtrack config [flags]
backtrack config [command]
```

管理备份包中的配置文件。支持查看、导出和导入备份包中的配置文件。

```bash
Flags:
  -b, --backup-config string   备份文件路径
  -v, --view-config string     要查看的配置文件名称(backup_config.yaml, file_map.yaml) (默认 "backup_config.yaml")

可用子命令:
  export      从备份包导出配置
  import      导入配置到备份包
```

#### export 子命令
```bash
backtrack config export [flags]

从备份包导出配置文件。

Flags:
  -c, --config string   要导出的配置文件名称(backup_config.yaml, file_map.yaml) (默认 "backup_config.yaml")
  -o, --output string   导出的配置文件路径

示例:
  # 从备份包导出配置
  backtrack config export --backup-config backup.zip --config backup_config.yaml --output my_config.yaml
```

#### import 子命令
```bash
backtrack config import [flags]

将配置文件导入到备份包。

Flags:
  -c, --config string   要替换的配置文件名称(backup_config.yaml, file_map.yaml) (默认 "backup_config.yaml")
  -i, --import string   要导入的配置文件路径
  -f, --force           强制替换

示例:
  # 将配置导入到备份包
  backtrack config import --backup-config backup.zip --config backup_config.yaml --import my_config.yaml
```

## 🏗️ 项目结构

```
back-track/
├── main.go          # 主程序入口
├── backup.go        # 备份功能实现
├── restore.go       # 还原功能实现
├── script.go        # 脚本执行功能
├── config.go        # 配置管理功能
├── tools.go         # 工具函数
├── config.yaml      # 配置文件示例
├── go.mod          # Go 模块定义
├── Taskfile.yml    # 构建任务配置
└── testdata/       # 测试数据
```

## 📦 依赖项

- [cobra](https://github.com/spf13/cobra): 命令行框架
- [progressbar](https://github.com/schollz/progressbar): 进度条显示
- [yaml.v3](https://gopkg.in/yaml.v3): YAML 解析
- [compress](https://github.com/klauspost/compress): 压缩算法

## 🧪 测试

```bash
# 运行测试
task test

# 构建二进制文件
task build
```

## 🔒 权限要求

BackTrack 需要 root 权限运行，以便能够：
- 访问系统文件
- 暂停和恢复 systemd 服务
- 在系统目录中创建文件

## 📝 注意事项

1. **备份文件格式**: 备份文件为 ZIP 格式，包含：
   - 原始文件数据
   - 配置文件备份 (`backup_config.yaml`)
   - 文件路径映射 (`file_map.yaml`)

2. **服务管理**: 仅支持 systemd 服务管理

3. **文件排除**: 支持精确目录名匹配和通配符文件模式匹配

4. **并发处理**: 自动根据 CPU 核心数设置并发工作线程

## 🤝 贡献

欢迎提交 Issue 和 Pull Request 来改进 BackTrack！

## 📄 许可证

MIT License