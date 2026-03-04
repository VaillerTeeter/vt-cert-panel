// 包：app | 用途：应用程序生命周期管理（初始化、启动、优雅关闭）| 关键接口/结构：App
// 关键职责：1）初始化所有服务（auth、cert、DNS、HTTP、renewal）| 2）启动 HTTP 服务器与后台续期 worker | 3）监听系统信号实现优雅关闭
package app

// === 依赖包导入 === | 说明：应用程序生命周期管理所需的核心包和内部模块
import (
    // context 包用于管理 goroutine 的生命周期和取消操作 | 用途：启动 worker 时创建上下文、优雅关闭时取消上下文
    // | 来自：Go 标准库
    "context"
    // errors 包提供错误处理功能 | 用途：判断 HTTP 服务器是否因 Shutdown 而正常关闭（http.ErrServerClosed）
    // | 来自：Go 标准库
    "errors"
    // net/http 包提供 HTTP 服务器功能 | 用途：处理 HTTP 错误类型（http.ErrServerClosed）
    // | 来自：Go 标准库
    "net/http"
    // os 包提供操作系统接口 | 用途：获取系统信号
    // | 来自：Go 标准库
    "os"
    // os/signal 包处理操作系统信号 | 用途：监听 SIGINT（Ctrl+C）和 SIGTERM（docker stop）信号进行优雅关闭
    // | 来自：Go 标准库
    "os/signal"
    // syscall 包定义系统信号常量 | 用途：定义 SIGINT 和 SIGTERM 信号常量
    // | 来自：Go 标准库
    "syscall"
    // time 包处理时间操作 | 用途：设置上下文超时（10 秒优雅关闭超时）、时间计算
    // | 来自：Go 标准库
    "time"
    // cert 包包含 ACME 证书相关功能 | 用途：创建 TechnitiumProvider 和 Service 实例用于证书申请和续期
    // | 来自：内部模块 vt-cert-panel/internal/cert
    "vt-cert-panel/internal/cert"
    // config 包包含配置文件加载和参数管理 | 用途：加载应用配置文件（yaml）、获取服务器地址、存储路径等
    // | 来自：内部模块 vt-cert-panel/internal/config
    "vt-cert-panel/internal/config"
    // db 包包含数据库初始化功能 | 用途：打开 SQLite 数据库连接
    // | 来自：内部模块 vt-cert-panel/internal/db
    "vt-cert-panel/internal/db"
    // dns 包包含 DNS 客户端实现 | 用途：创建 TechnitiumClient 实例用于 DNS 记录操作
    // | 来自：内部模块 vt-cert-panel/internal/dns
    "vt-cert-panel/internal/dns"
    // httpx 包包含 HTTP 服务器实现 | 用途：创建 HTTP 服务器实例，处理所有 Web 路由和 API 端点
    // | 来自：内部模块 vt-cert-panel/internal/httpx
    "vt-cert-panel/internal/httpx"
    // repository 包包含数据访问层实现 | 用途：创建数据库访问对象（UserRepository、SessionRepository、CertificateRepository）
    // | 来自：内部模块 vt-cert-panel/internal/repository
    "vt-cert-panel/internal/repository"
    // service 包包含业务逻辑层实现 | 用途：创建各种服务实例（AuthService、CertificateService、RenewalWorker）
    // | 来自：内部模块 vt-cert-panel/internal/service
    "vt-cert-panel/internal/service"
)

// App 表示应用程序的主实例 | 关键字段：httpServer = HTTP 服务器、renewal = 后台续期 worker
type App struct {
    // httpServer 表示 HTTP 服务器实例，用于处理所有 Web 请求和 API | 类型：*httpx.Server | 作用域：内部使用，不暴露给外部
    httpServer *httpx.Server
    // renewal 表示后台续期 worker，定期检查并续期即将过期的证书 | 类型：*service.RenewalWorker | 作用域：内部使用，不暴露给外部
    renewal    *service.RenewalWorker
}

// New 用于创建并初始化应用程序实例 | 输入：无（从环境变量读取配置）| 输出：返回初始化完成的 *App 实例或错误
// 可能失败：1）关键环境变量缺失或格式错误 | 2）数据库连接失败 | 3）HTTP 服务器创建失败
// 初始化流程：加载配置 -> 打开数据库 -> 创建各服务（auth、cert、DNS、renewal）-> 创建 HTTP 服务器 -> 返回 App 实例
func New() (*App, error) {
    // 加载环境变量配置，返回 Config 结构体或错误
    // Config 包含服务器地址、存储路径、ACME 和 Technitium 配置等
    cfg, err := config.Load()
    if err != nil {
        // 如果加载配置失败（比如缺少必填环境变量），返回错误，应用无法启动
        return nil, err
    }

    // 打开 SQLite 数据库连接
    // SQLite 中存储用户、会话、证书等数据
    sqlDB, err := db.Open(cfg.Storage.SQLitePath)
    if err != nil {
        // 如果数据库连接失败，返回错误
        return nil, err
    }

    // 初始化所有数据访问层 repository
    // 每个 repository 用于数据库表的 CRUD 操作
    users := repository.NewUserRepository(sqlDB)
    sessions := repository.NewSessionRepository(sqlDB)
    certRepo := repository.NewCertificateRepository(sqlDB)
    // 初始化业务逻辑层服务
    // authSvc：处理用户认证、会话管理
    authSvc := service.NewAuthService(cfg, users, sessions)
    // dnsClient：与 Technitium DNS API 交互
    dnsClient := dns.NewTechnitiumClient(cfg.Technitium)
    // provider：实现 lego DNS-01 完整性验证接口
    provider := cert.NewTechnitiumProvider(dnsClient, cfg.Technitium.PropagationSec)
    // issuer：使用 lego 库和 Technitium 进行 ACME 证书申请
    issuer := cert.NewService(cfg, provider)
    // certSvc：业务层证书管理服务（申请、查询、续期）
    certSvc := service.NewCertificateService(certRepo, issuer)
    // renewal：后台 worker，定期检查并自动续期即将过期的证书
    renewal := service.NewRenewalWorker(cfg, certSvc)

    // 创建 HTTP 服务器
    // 服务器包含所有路由：/api/bootstrap、/api/login、/api/certificates 等
    httpServer, err := httpx.NewServer(cfg.Server.Address, authSvc, certSvc)
    if err != nil {
        // 如果 HTTP 服务器创建失败，返回错误
        return nil, err
    }

    // 返回初始化完成的应用程序实例
    return &App{httpServer: httpServer, renewal: renewal}, nil
}

// (a *App) Run 用于启动应用程序服务（HTTP 服务器和后台 worker）| 输入：无 | 输出：返回错误或 nil（优雅关闭成功）
// 可能失败：1）HTTP 服务器监听失败 | 2）后台 worker 异常 | 3）信号处理异常
// 执行流程：启动后台 worker -> 启动 HTTP 服务器 -> 监听系统信号 -> 优雅关闭
// 信号处理：监听 SIGINT（Ctrl+C）和 SIGTERM 信号，收到信号后关闭 HTTP 服务器并等待完成，超时 10 秒
func (a *App) Run() error {
    // 创建主程序上下文，用于控制后台 worker 的生命周期
    // 当接收到中断信号时，cancel() 会被调用来通知 worker 关闭
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 启动后台续期 worker
    // worker 会定期检查证书过期时间，并在需要时自动续期
    go a.renewal.Start(ctx)

    // 创建错误通道，用于接收 HTTP 服务器的错误信息
    // 缓冲大小为 1，避免 goroutine 泄露
    errCh := make(chan error, 1)
    // 在独立的 goroutine 中启动 HTTP 服务器
    // 服务器会在这里阻塞，直到接收到关闭信号或发生致命错误
    go func() {
        errCh <- a.httpServer.Run()
    }()

    // 创建信号通道，用于接收操作系统信号
    // 缓冲大小为 1，避免信号丢失
    sigCh := make(chan os.Signal, 1)
    // 注册两个系统信号
    // SIGINT：用户按下 Ctrl+C 时触发
    // SIGTERM：docker stop 等命令发送的信号
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    // 使用 select 监听多个通道，实现优雅关闭
    select {
    case sig := <-sigCh:
        // 收到系统信号
        _ = sig
        // 创建关闭上下文，设置 10 秒超时
        // 如果 HTTP 服务器在 10 秒内无法关闭，将被强制关闭
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer shutdownCancel()
        // 取消后台 worker 的上下文，通知它停止运行
        cancel()
        // 优雅关闭 HTTP 服务器
        // 服务器会停止接收新连接，等待现有请求完成后关闭
        return a.httpServer.Shutdown(shutdownCtx)
    case err := <-errCh:
        // HTTP 服务器发生错误或关闭
        // http.ErrServerClosed 是正常情况（外部调用 Shutdown）
        if errors.Is(err, http.ErrServerClosed) {
            // 正常关闭，返回 nil
            return nil
        }
        // 其他错误，向调用者返回
        return err
    }
}
