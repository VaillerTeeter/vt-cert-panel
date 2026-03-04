# vt-cert-panel

中文 Web UI 的单体证书面板，支持 ACME 证书申请/管理/下载，使用 DNS-01 并对接 Technitium DNS API。

## 功能

- 首次启动初始化管理员（单用户）
- 管理员登录后申请证书（单域名/多域名/通配符）
- 证书列表展示（主域名、域名列表、状态、到期时间）
- 下载证书 ZIP（`fullchain.pem` + `privkey.pem`）
- 自动续期任务（按配置定时检查，到期前自动续期）

## 技术栈

- 后端：Go
- 前端：原生 HTML/CSS/JS（中文）
- ACME：lego
- 数据库：SQLite
- 部署：Docker（docker build）

## 本地开发

### 开发快速开始（Windows）

使用统一脚本 `vt.ps1` 完成所有操作：

```powershell
# 1) 首次初始化：设置环境变量并初始化
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Command env `
  -BaseUrl "http://192.168.1.100:5380" `
  -Token "your-api-token" `
  -AcmeEmail "admin@example.com"

# 2) 启动开发服务
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Command debug

# 3) 访问 http://localhost:8080
```

### 脚本详解（`scripts/vt.ps1`）

#### 概述

`vt.ps1` 是 vt-cert-panel 的统一开发入口，支持以下 4 个子命令：

| 命令 | 功能 | 用途 |
|-----|------|-----|
| `env` | 设置环境变量并执行完整初始化 | 首次开发环境准备 |
| `init` | 仅初始化开发环境（不改变环境变量） | 重新初始化或部分初始化 |
| `debug` | 启动开发服务（可自动打开浏览器） | 本地调试与开发 |
| `docker` | 构建 Docker 镜像（使用语义化版本号） | 容器镜像构建与部署 |

#### 脚本内部工作流

```
用户输入 -Command
    ↓
验证参数完整性
    ↓
┌─────────────────────────────────────────────────────────┐
│ env 子命令                                              │
├─────────────────────────────────────────────────────────┤
│ 1. 验证 -BaseUrl, -Token, -AcmeEmail 参数              │
│ 2. 设置进程环境变量 (VT_CERT_*)                        │
│ 3. 自动执行 init 命令完成初始化                        │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ init 子命令（核心初始化流程）                          │
├─────────────────────────────────────────────────────────┤
│ 1. 验证必填环境变量 (TECHNITIUM_BASE_URL/TOKEN/EMAIL)  │
│ 2. 检测/安装工具 (Git/Go/Docker via winget)            │
│ 3. 创建数据目录 (data/data-certs/data-acme/bin)       │
│ 4. 创建/更新 .env.local 配置文件                       │
│ 5. 执行 go mod tidy 下载依赖（可选 -SkipGoMod）        │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ debug 子命令（开发服务模式）                           │
├─────────────────────────────────────────────────────────┤
│ 1. 设置日志级别为 DEBUG                                │
│ 2. 后台启动浏览器打开 http://localhost:8080            │
│ 3. 前台运行 go run ./cmd/server                        │
│ 4. Ctrl+C 停止服务，自动清理后台浏览器任务            │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ docker 子命令（容器镜像构建）                          │
├─────────────────────────────────────────────────────────┤
│ 1. 验证 -Version 参数（格式 x.y.z，如 1.0.0）         │
│ 2. 执行 docker build --no-cache -t vt-cert-panel:x.y.z │
│ 3. 镜像可用 docker run 或 docker-compose 部署         │
└─────────────────────────────────────────────────────────┘
```

#### 全部参数说明

**全局参数**（适用于所有命令）：
- `-Command <string>`：子命令名（env/init/debug/docker），位置参数，可省略 `-Command`
- `-Help`：显示帮助信息并退出
- `-SkipToolInstall`：跳过 winget 工具安装（Git/Go/Docker）
- `-SkipDocker`：跳过 Docker 检测与安装
- `-SkipGoMod`：跳过 go mod tidy

**`env` 命令参数**（必填）：
- `-BaseUrl <string>`：Technitium DNS API 的基础 URL（例：`http://192.168.1.100:5380`）
- `-Token <string>`：Technitium DNS API 的认证令牌
- `-AcmeEmail <string>`：ACME 账户注册邮箱（如 `admin@example.com`）

**`docker` 命令参数**（必填）：
- `-Version <string>`：Docker 镜像版本号，必须为语义化版本格式 `x.y.z`（如 `1.0.0`）

#### 子命令使用示例

##### `env` - 首次环境初始化

```powershell
# 完整示例：设置所有必填变量并立即初始化
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Command env `
  -BaseUrl "http://192.168.1.100:5380" `
  -Token "m2e7f3c9a8b1d4e6f" `
  -AcmeEmail "admin@example.com"

# 脚本执行流程：
# 1. 验证 BaseUrl/Token/AcmeEmail 非空
# 2. 设置进程环境变量 VT_CERT_TECHNITIUM_BASE_URL 等
# 3. 自动调用 init 流程
#    - 检测/安装 Git、Go、Docker
#    - 创建 data/ 等目录
#    - 生成 .env.local 文件
#    - 执行 go mod tidy
# 4. 完成后退出（LASTEXITCODE=0）
```

**环境变量说明**：
- `VT_CERT_TECHNITIUM_BASE_URL`：Technitium DNS 的 HTTP 地址
- `VT_CERT_TECHNITIUM_TOKEN`：API 令牌（从 Technitium Web UI 获取）
- `VT_CERT_ACME_USER_EMAIL`：Let's Encrypt ACME 账户邮箱（会发送证书过期提醒）

##### `init` - 检测与初始化开发环境

```powershell
# 基础初始化（使用已有环境变量）
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 init

# 跳过工具安装（本机已有 Git/Go/Docker）
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 init -SkipToolInstall

# 跳过 Docker 检测（不需要容器开发）
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 init -SkipDocker

# 跳过 go mod tidy（离线开发或已经下载依赖）
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 init -SkipGoMod

# 组合使用：跳过工具安装和 Docker
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 init -SkipToolInstall -SkipDocker
```

**检测的目录和文件**：
- 检测并创建：`data/`、`data/certs/`、`data/acme/`、`bin/`
- 创建或更新：`.env.local`（若不存在则从 `.env.local.template` 复制或使用默认值）

**go mod tidy 的用途**：
- 从 `go.mod` 中下载声明的所有依赖包
- 删除 `go.mod` 中未实际使用的依赖
- 更新 `go.sum`（依赖版本锁定文件）

##### `debug` - 启动开发服务

```powershell
# 最简单的使用方式
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 debug

# 自动打开浏览器，访问 http://localhost:8080
# 日志级别自动设置为 DEBUG（详细日志），便于调试
# 按 Ctrl+C 停止服务
```

**工作过程**：
1. 设置 `VT_CERT_LOG_LEVEL=debug`
2. 在后台启动 PowerShell 任务，2 秒后打开浏览器到 `http://localhost:8080`
3. 前台运行 `go run ./cmd/server`，实时输出服务日志
4. 按 Ctrl+C 停止服务，后台浏览器任务自动清理

**故障排查**：
- 若浏览器未能打开，可手动访问 `http://localhost:8080`
- 若提示"端口被占用"，说明有其他进程占用 8080 端口，可修改 `cmd/server` 中的端口配置

##### `docker` - 构建镜像

```powershell
# 构建版本 1.0.0 镜像
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 docker -Version "1.0.0"

# 构建版本 1.2.3 镜像
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 docker -Version "1.2.3"

# 错误示例（会被拒绝，需符合 x.y.z 格式）：
# powershell ... docker -Version "1.0"        # 错：缺少 patch 版本
# powershell ... docker -Version "1.0.0-rc"   # 错：不支持预发布标签
```

**构建细节**：
- `--no-cache`：跳过 Docker 缓存，确保每次都是完全新构建
- `--progress=plain`：纯文本进度输出，适合 CI/CD 环境和日志记录
- 最终镜像标签：`vt-cert-panel:1.0.0`（可用 `docker run` 运行）

**后续部署**：
```bash
# 查看已构建的镜像
docker images | grep vt-cert-panel

# 运行镜像（映射端口、挂载数据目录）
docker run -d -p 8080:8080 -v $(pwd)/data:/app/data vt-cert-panel:1.0.0

# 使用 docker-compose （若有 docker-compose.yml）
docker-compose up -d
```

#### 帮助命令

```powershell
# 显示脚本帮助
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Help

# 或者（省略 -Command）
powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Help
```

#### 常见问题

**Q1: 脚本报错"缺少必填参数"**
```
Error: env command requires -BaseUrl, -Token and -AcmeEmail
```
**A:** 确保使用 `-Command env` 时提供了全部三个参数：`-BaseUrl`、`-Token`、`-AcmeEmail`

**Q2: PowerShell 提示"无法识别 go 命令"**
```
go : 无法将"go"项识别为 cmdlet、函数、脚本文件或可运行程序的名称
```
**A:** 刷新 PowerShell 的 PATH 环境变量：
```powershell
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
go version
```

**Q3: go mod tidy 失败，报"missing go.sum entry"**
```
missing go.sum entry for module providing package ...
```
**A:** 手动执行依赖管理：
```powershell
go mod tidy
go mod download
go run ./cmd/server
```

**Q4: Docker 构建失败**

**A:** 检查：
- Docker Desktop 已启动
- Dockerfile 路径正确（应在项目根目录）
- 足够的磁盘空间
- 网络连接正常（需下载基础镜像）

#### 脚本内部结构

脚本采用 **函数式设计**，包含以下核心函数：

| 函数 | 用途 |
|------|-----|
| `Write-Step` / `Write-Ok` / `Write-Err` / `Write-WarnMsg` | 彩色输出控制台信息 |
| `Show-Help` | 输出脚本帮助文本 |
| `Test-Tool` | 检测系统 PATH 中是否已安装工具 |
| `Ensure-WingetPackage` | 通过 winget 检测并自动安装工具 |
| `Set-EnvFileValue` | 在 .env 文件中设置或更新键值对（支持追加新变量） |
| `Validate-RequiredEnv` | 验证必填环境变量是否已设置 |
| `Invoke-EnvCommand` | env 子命令实现 |
| `Invoke-InitCommand` | init 子命令实现（核心初始化逻辑） |
| `Invoke-DebugCommand` | debug 子命令实现 |
| `Invoke-DockerCommand` | docker 子命令实现 |

#### 技术细节

**编码与行尾规范**：
- 文件编码：UTF-8（无 BOM）
- 行尾符：LF（Unix 格式）
- 缩进：4 个空格（禁止 Tab）

**错误处理**：
- 严格模式（`Set-StrictMode -Version Latest`）：禁止使用未声明的变量等
- 全局错误处理（`$ErrorActionPreference = 'Stop'`）：遇到错误立即停止

**环境变量作用域**：
- `$env:*`：进程级别（当前 PowerShell 进程及其子进程可见）
- `.env.local`：文件持久化（后续 shell 会话可读取）

## 首次使用

1. 打开页面后先进行管理员初始化
2. 使用管理员账号登录
3. 在"证书申请"里输入域名列表和邮箱后申请
4. 在"证书管理"里下载 ZIP 证书包

## 运行与调试

使用 `vt.ps1 -Command debug` 启动开发服务，访问 `http://localhost:8080`。

## Docker 部署

### Docker 镜像构建

使用 `vt.ps1 -Command docker -Version "x.y.z"` 构建镜像。

## 注意事项

- 脚本位置：`scripts/vt.ps1` 是唯一的开发脚本（所有功能已集成）
- 证书与数据库均保存在挂载目录 `data/`，容器重建不会丢失

## 常见问题排查

### 1) PowerShell 提示 `go` 无法识别

现象：

```text
go : 无法将"go"项识别为 cmdlet、函数、脚本文件或可运行程序的名称
```

处理：

```powershell
# 刷新当前终端 PATH（无需重启系统）
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
go version
```

如果系统终端可用、VS Code 终端不可用，请在 VS Code 用户设置中确认：

- `terminal.integrated.inheritEnv` 为 `true`
- `terminal.integrated.env.windows.PATH` 已包含 `C:\Program Files\Go\bin`

### 2) 运行前报错 `missing go.sum entry`

现象：

```text
missing go.sum entry for module providing package ...
```

处理：

```powershell
go mod tidy
go mod download
go run ./cmd/server
```
