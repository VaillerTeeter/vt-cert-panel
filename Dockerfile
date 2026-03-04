# 文件：Dockerfile | 用途：证书管理应用的 Docker 镜像构建配置 | 构建阶段：builder（编译）、runtime（运行） | 最终镜像大小：约 50-80MB

# === 阶段：builder === | 用途：编译 Go 源代码生成可执行二进制文件 | 基础镜像：golang:1.23-alpine | 输出产物：/out/vt-cert-panel（二进制可执行文件）
# FROM 指令 构建编译环境 | 输入：golang:1.23 官方镜像（包含 Go 编译工具链）| 输出：builder 阶段的基础环境
# 选择原因：Alpine 版本轻量级（约 360MB），包含所有 Go 构建工具，相比标准 golang:1.23 镜像（约 800MB）节省空间
FROM golang:1.23-alpine AS builder
# WORKDIR 指令 设置工作目录 | 输入：无 | 输出：后续指令在 /app 目录执行
# 含义：创建或切换到 /app 目录，所有后续 RUN、COPY 等命令都在此目录执行
WORKDIR /app
# COPY 指令 复制依赖文件 | 输入：宿主机 go.mod 文件 | 输出：容器内 /app/go.mod
# 目的：先复制依赖文件单独构建，利用 Docker 分层缓存加快构建（如果源代码变更但依赖未变，可复用此层）
COPY go.mod .
# RUN 指令 下载依赖 | 输入：go.mod 文件 | 输出：下载的 Go 模块到容器缓存
# 命令 go mod download：读取 go.mod 文件，下载所有声明的依赖到本地缓存（通常位于 $GOPATH/pkg/mod）
RUN go mod download
# COPY 指令 复制完整源代码 | 输入：宿主机完整项目代码 | 输出：容器内 /app 目录
# 目的：复制所有源代码文件以便编译（此时依赖已缓存，无需重复下载，可加速构建）
COPY . .
# RUN 指令 编译 Go 程序 | 输入：/app 目录的 Go 源代码 | 输出：/out/vt-cert-panel 二进制文件
# 命令拆解：
#   CGO_ENABLED=0 - 禁用 C 语言调用（保证跨平台兼容性）
#   GOOS=linux GOARCH=amd64 - 指定编译目标操作系统为 Linux、架构为 AMD64（x86-64 64位）
#   go build -o /out/vt-cert-panel ./cmd/server - 编译 cmd/server 包，输出二进制到 /out 目录
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/vt-cert-panel ./cmd/server

# === 阶段：runtime === | 用途：运行已编译的应用程序（精简运行时环境） | 基础镜像：alpine:3.20 | 输出产物：可运行的应用容器
# FROM 指令 运行时基础镜像 | 输入：alpine:3.20 官方镜像（轻量级 Linux 发行版）| 输出：运行时环境
# 选择原因：Alpine 镜像极度精简（约 7MB），仅包含必要系统工具，相比 ubuntu 或 debian 镜像（约 100-200MB）节省 95% 空间
# 版本说明：3.20 是长期支持版本，确保安全补丁及时更新
FROM alpine:3.20
# RUN 指令 创建非 root 用户 | 输入：无 | 输出：appuser 用户，UID=10001
# 命令 adduser：创建系统用户（非 root）以增强容器安全性（即使容器被破解也无法获得完整权限）
#   -D：不设置密码（无法用密码登录，只能通过容器初始化时指定）
#   -u 10001：指定用户 ID（避免与系统用户冲突）
RUN adduser -D -u 10001 appuser
# WORKDIR 指令 设置运行时工作目录 | 输入：无 | 输出：运行时工作目录为 /app
WORKDIR /app
# COPY 指令 从 builder 阶段复制二进制文件 | 输入：builder 阶段的 /out/vt-cert-panel | 输出：运行时容器内 /app/vt-cert-panel
# 方式 --from=builder：从前面的 builder 阶段复制（多阶段构建的关键指令）
# 意义：仅复制最终的二进制文件，丢弃整个编译工具链，大幅减小运行时镜像体积
COPY --from=builder /out/vt-cert-panel /app/vt-cert-panel
# COPY 指令 复制前端静态资源 | 输入：宿主机 web 目录（HTML、CSS、JavaScript） | 输出：容器内 /app/web
# 用途：应用需要提供前端界面文件（HTML、CSS、JS），宿主机访问时通过 HTTP 服务返回这些文件
COPY web /app/web
# RUN 指令 创建数据目录并设置权限 | 输入：无 | 输出：/app/data 目录的所有权修改为 appuser
# 命令拆解：
#   mkdir -p /app/data - 创建数据存储目录（-p 标志确保父目录不存在时自动创建）
#   chown -R appuser:appuser /app - 递归修改 /app 目录及所有子文件的所有者为 appuser（确保非 root 用户可读写）
RUN mkdir -p /app/data && chown -R appuser:appuser /app
# USER 指令 切换运行用户 | 输入：appuser（刚创建的非 root 用户）| 输出：后续指令以及容器启动时使用 appuser 身份
# 安全意义：应用不再以 root 权限运行，即使容器被破解攻击者也无法获得系统 root 权限
USER appuser
# EXPOSE 指令 暴露端口 | 用途：声明应用监听的 HTTP 服务端口 | 访问：docker run -p 8080:8080 或 docker-compose ports 映射
# 说明：此指令仅声明端口信息（用于文档和容器间通信），实际端口映射需在 docker run 或 docker-compose 中配置
EXPOSE 8080
# VOLUME 指令 声明卷挂载点 | 用途：将容器内目录标记为卷，支持在容器删除后数据持久化 | 宿主机挂载建议：docker-compose 中配置 volumes 或 docker run -v
# 含义：
#   /app/data - SQLite 数据库文件和申请的证书文件（允许宿主机挂载本地目录或命名卷）
VOLUME ["/app/data"]
# ENTRYPOINT 指令 启动命令 | 用途：容器启动时执行应用程序 | 参数含义：/app/vt-cert-panel 是二进制可执行文件路径
# 说明：ENTRYPOINT 定义容器启动时的主进程，容器退出时此进程停止，这是应用在容器中的入口点
ENTRYPOINT ["/app/vt-cert-panel"]
