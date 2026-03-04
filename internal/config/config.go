// 包：config | 用途：环境变量配置加载与参数管理 | 关键函数：Load、defaultConfig、SessionTTL
// 关键职责：1）从环境变量加载配置 | 2）提供配置默认值 | 3）创建必要的目录结构 | 4）配置参数验证
package config

// === 依赖包导入 === | 说明：配置文件加载和参数管理所需的核心包
import (
    // fmt 包提供格式化输出功能 | 用途：错误消息格式化（fmt.Errorf）
    // | 来自：Go 标准库
    "fmt"
    // os 包提供操作系统接口 | 用途：读取配置文件（os.ReadFile）、创建目录（os.MkdirAll）
    // | 来自：Go 标准库
    "os"
    // path/filepath 包处理文件路径 | 用途：构建跨平台的文件路径（filepath.Join）
    // | 来自：Go 标准库
    "path/filepath"
    // strconv 包提供字符串转换功能 | 用途：解析环境变量中的整数与布尔值
    // | 来自：Go 标准库
    "strconv"
    // strings 包提供字符串处理功能 | 用途：去除空白字符（strings.TrimSpace）
    // | 来自：Go 标准库
    "strings"
    // time 包处理时间操作 | 用途：时间转换（time.Hour - 将小时转换为 time.Duration）
    // | 来自：Go 标准库
    "time"
)

// Config 表示应用程序的完整配置对象 | 关键字段：Server、Storage、AuthAcme、Technitium、AutoRenewal
type Config struct {
    // Server | 服务器配置（监听地址、端口等）| 类型：ServerConfig | 作用域：HTTP 服务器初始化
    Server      ServerConfig      `yaml:"server"`
    // Storage | 存储配置（数据库、证书、密钥路径）| 类型：StorageConfig | 作用域：数据库和文件操作
    Storage     StorageConfig     `yaml:"storage"`
    // Auth | 认证配置（会话 TTL 等）| 类型：AuthConfig | 作用域：认证服务
    Auth        AuthConfig        `yaml:"auth"`
    // ACME | ACME 配置（Let's Encrypt 相关）| 类型：ACMEConfig | 作用域：证书申请服务
    ACME        ACMEConfig        `yaml:"acme"`
    // Technitium | Technitium DNS 配置（API 地址、token 等）| 类型：TechnitiumConfig | 作用域：DNS 操作
    Technitium  TechnitiumConfig  `yaml:"technitium"`
    // AutoRenewal | 自动续期配置（检查间隔、续期窗口）| 类型：AutoRenewalConfig | 作用域：续期 worker
    AutoRenewal AutoRenewalConfig `yaml:"autoRenewal"`
}

// ServerConfig 表示 HTTP 服务器的配置
type ServerConfig struct {
    // Address | 监听地址与端口（如：:8080、127.0.0.1:3000）| 类型：string | 作用域：HTTP 服务器创建
    Address string `yaml:"address"`
}

// StorageConfig 表示数据存储相关的配置
type StorageConfig struct {
    // DataDir | 数据根目录，存储所有程序生成的文件 | 类型：string | 作用域：所有数据操作
    DataDir       string `yaml:"dataDir"`
    // SQLitePath | SQLite 数据库文件路径 | 类型：string | 作用域：数据库连接
    SQLitePath    string `yaml:"sqlitePath"`
    // CertsDir | SSL/TLS 证书存储目录 | 类型：string | 作用域：证书文件操作
    CertsDir      string `yaml:"certsDir"`
    // AccountKeyDir | ACME 账户私钥存储目录 | 类型：string | 作用域：ACME 操作（敏感数据）
    AccountKeyDir string `yaml:"accountKeyDir"`
}

// AuthConfig 表示认证相关的配置
type AuthConfig struct {
    // SessionTTLHours | 用户会话有效期（小时）| 类型：int | 作用域：会话管理
    SessionTTLHours int `yaml:"sessionTtlHours"`
}

// ACMEConfig 表示 Let's Encrypt ACME 相关的配置
type ACMEConfig struct {
    // DirectoryURL | ACME 服务器目录 URL | 类型：string | 作用域：lego 客户端初始化
    DirectoryURL string `yaml:"directoryUrl"`
    // UserEmail | ACME 账户邮箱（Let's Encrypt 通知地址）| 类型：string | 作用域：证书申请、账户注册
    UserEmail    string `yaml:"userEmail"`
}

// TechnitiumConfig 表示 Technitium DNS 相关的配置
type TechnitiumConfig struct {
    // BaseURL | Technitium DNS API 服务地址 | 类型：string | 作用域：DNS 客户端初始化
    BaseURL        string `yaml:"baseUrl"`
    // Token | Technitium API 认证 token | 类型：string | 作用域：DNS API 请求（敏感数据）
    Token          string `yaml:"token"`
    // DefaultTTL | DNS 记录 TTL（秒）| 类型：int | 作用域：DNS 记录添加
    DefaultTTL     int    `yaml:"defaultTtl"`
    // PropagationSec | DNS 传播等待时间（秒）| 类型：int | 作用域：ACME 验证
    PropagationSec int    `yaml:"propagationSec"`
}

// AutoRenewalConfig 表示证书自动续期相关的配置
type AutoRenewalConfig struct {
    // Enabled | 是否启用自动续期 | 类型：bool | 作用域：续期 worker 启动
    Enabled           bool `yaml:"enabled"`
    // CheckIntervalHour | 检查间隔（小时）| 类型：int | 作用域：续期 worker 时间表
    CheckIntervalHour int  `yaml:"checkIntervalHour"`
    // RenewBeforeDays | 提前多少天开始续期 | 类型：int | 作用域：续期判断逻辑
    RenewBeforeDays   int  `yaml:"renewBeforeDays"`
}

// Load 用于从环境变量加载配置并初始化相关目录
// 输入：无
// 输出：返回解析后的 *Config 或错误
// 可能失败：1）关键环境变量缺失 | 2）环境变量格式错误 | 3）目录创建失败
// 执行流程：加载默认值 -> 环境变量覆盖 -> 参数校验 -> 创建目录 -> 返回 Config
func Load() (*Config, error) {
    // 首先使用默认配置作为基础
    cfg := defaultConfig()

    // === 应用配置默认值（确保关键字段不为空）=== | 用途：如果默认值被异常覆盖，使用硬编码兜底
    // 优先级：环境变量的值 > Load 函数中的硬编码默认值 > defaultConfig 中的默认值

    // Server.Address 检查 | 监听地址是否设置
    if cfg.Server.Address == "" {
        // 如果为空，使用默认值 ":8080"（监听所有接口的 8080 端口）
        cfg.Server.Address = ":8080"
    }

    // Storage.DataDir 检查 | 数据根目录是否设置
    if cfg.Storage.DataDir == "" {
        // 如果为空，使用默认值 "./data"（当前目录下的 data 文件夹）
        cfg.Storage.DataDir = "./data"
    }

    // Storage.SQLitePath 检查 | 数据库文件路径是否设置
    if cfg.Storage.SQLitePath == "" {
        // 如果为空，使用默认路径：<DataDir>/app.db（在数据目录下创建 app.db）
        // filepath.Join 会自动处理跨平台路径分隔符（/ 或 \）
        cfg.Storage.SQLitePath = filepath.Join(cfg.Storage.DataDir, "app.db")
    }

    // Storage.CertsDir 检查 | 证书存储目录是否设置
    if cfg.Storage.CertsDir == "" {
        // 如果为空，使用默认路径：<DataDir>/certs（在数据目录下创建 certs 子目录）
        cfg.Storage.CertsDir = filepath.Join(cfg.Storage.DataDir, "certs")
    }

    // Storage.AccountKeyDir 检查 | ACME 账户密钥存储目录是否设置
    if cfg.Storage.AccountKeyDir == "" {
        // 如果为空，使用默认路径：<DataDir>/acme（在数据目录下创建 acme 子目录）
        // 此目录存储 Let's Encrypt 账户私钥，权限应严格控制（0o600）
        cfg.Storage.AccountKeyDir = filepath.Join(cfg.Storage.DataDir, "acme")
    }

    // Auth.SessionTTLHours 检查 | 会话有效期是否有效（大于 0）
    if cfg.Auth.SessionTTLHours <= 0 {
        // 如果为 0 或负数，使用默认值 24（小时）
        // 确保会话不会因配置错误而永不过期
        cfg.Auth.SessionTTLHours = 24
    }

    // Technitium.DefaultTTL 检查 | DNS 记录 TTL 是否有效
    if cfg.Technitium.DefaultTTL <= 0 {
        // 如果为 0 或负数，使用默认值 120（秒）
        // 120 秒是 ACME 验证的标准 TTL 值，足以完成验证又不会过期
        cfg.Technitium.DefaultTTL = 120
    }

    // Technitium.PropagationSec 检查 | DNS 传播等待时间是否有效
    if cfg.Technitium.PropagationSec <= 0 {
        // 如果为 0 或负数，使用默认值 120（秒）
        // 等待 DNS 记录全球传播，减少 ACME 验证失败的风险
        cfg.Technitium.PropagationSec = 120
    }

    // AutoRenewal.CheckIntervalHour 检查 | 证书检查间隔是否有效
    if cfg.AutoRenewal.CheckIntervalHour <= 0 {
        // 如果为 0 或负数，使用默认值 24（小时）
        // 每 24 小时检查一次，足够频繁且资源消耗小
        cfg.AutoRenewal.CheckIntervalHour = 24
    }

    // AutoRenewal.RenewBeforeDays 检查 | 证书续期提前天数是否有效
    if cfg.AutoRenewal.RenewBeforeDays <= 0 {
        // 如果为 0 或负数，使用默认值 30（天）
        // 在证书过期前 30 天开始续期，确保足够的安全窗口
        // 避免因延迟导致证书真正过期导致服务中断
        cfg.AutoRenewal.RenewBeforeDays = 30
    }

    // 使用环境变量覆盖配置文件中的值（环境变量优先级最高）
    if err := applyEnvOverrides(&cfg); err != nil {
        return nil, err
    }

    // 校验运行所需的关键配置（缺少即启动失败）
    if err := validateRequired(cfg); err != nil {
        return nil, err
    }

    // === 创建必要的目录结构 === | 用途：确保数据存储目录存在
    // 创建根数据目录，权限 755（所有者读写执行，其他用户读执行）
    if err := os.MkdirAll(cfg.Storage.DataDir, 0o755); err != nil {
        return nil, fmt.Errorf("创建数据目录失败: %w", err)
    }

    // 创建证书存储目录
    if err := os.MkdirAll(cfg.Storage.CertsDir, 0o755); err != nil {
        return nil, fmt.Errorf("创建证书目录失败: %w", err)
    }

    // 创建 ACME 账户密钥存储目录
    if err := os.MkdirAll(cfg.Storage.AccountKeyDir, 0o755); err != nil {
        return nil, fmt.Errorf("创建ACME目录失败: %w", err)
    }

    // 配置加载完成，返回最终的 Config 对象
    return &cfg, nil
}

// applyEnvOverrides 用于将环境变量映射到配置对象（环境变量优先） | 输入：cfg = 配置对象指针 | 输出：返回错误或 nil（成功）
// 可能失败：环境变量值格式错误（非整数、非布尔值）时返回格式错误
func applyEnvOverrides(cfg *Config) error {
    // 读取 VT_CERT_SERVER_ADDRESS 环境变量 | 含义：HTTP 服务器监听地址
    if v := strings.TrimSpace(os.Getenv("VT_CERT_SERVER_ADDRESS")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Server.Address
        cfg.Server.Address = v
    }

    // === 存储相关环境变量处理 === | 用途：覆盖数据目录、数据库路径、证书目录、密钥目录配置

    // 读取 VT_CERT_STORAGE_DATA_DIR 环境变量 | 含义：数据根目录路径
    if v := strings.TrimSpace(os.Getenv("VT_CERT_STORAGE_DATA_DIR")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Storage.DataDir
        cfg.Storage.DataDir = v
    }
    // 读取 VT_CERT_STORAGE_SQLITE_PATH 环境变量 | 含义：SQLite 数据库文件路径
    if v := strings.TrimSpace(os.Getenv("VT_CERT_STORAGE_SQLITE_PATH")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Storage.SQLitePath
        cfg.Storage.SQLitePath = v
    }
    // 读取 VT_CERT_STORAGE_CERTS_DIR 环境变量 | 含义：证书存储目录路径
    if v := strings.TrimSpace(os.Getenv("VT_CERT_STORAGE_CERTS_DIR")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Storage.CertsDir
        cfg.Storage.CertsDir = v
    }
    // 读取 VT_CERT_STORAGE_ACCOUNT_KEY_DIR 环境变量 | 含义：ACME 账户密钥存储目录路径
    if v := strings.TrimSpace(os.Getenv("VT_CERT_STORAGE_ACCOUNT_KEY_DIR")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Storage.AccountKeyDir
        cfg.Storage.AccountKeyDir = v
    }

    // === ACME 相关环境变量处理 === | 用途：覆盖 Let's Encrypt 服务地址和账户邮箱

    // 读取 VT_CERT_ACME_DIRECTORY_URL 环境变量 | 含义：ACME 服务器目录 URL
    if v := strings.TrimSpace(os.Getenv("VT_CERT_ACME_DIRECTORY_URL")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 ACME.DirectoryURL
        cfg.ACME.DirectoryURL = v
    }
    // 读取 VT_CERT_ACME_USER_EMAIL 环境变量 | 含义：Let's Encrypt 账户邮箱（必填）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_ACME_USER_EMAIL")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 ACME.UserEmail
        cfg.ACME.UserEmail = v
    }

    // === Technitium DNS API 相关环境变量处理 === | 用途：覆盖 DNS 服务地址、API token、DNS TTL、传播时间

    // 读取 VT_CERT_TECHNITIUM_BASE_URL 环境变量 | 含义：Technitium DNS API 服务地址（必填）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_TECHNITIUM_BASE_URL")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Technitium.BaseURL
        cfg.Technitium.BaseURL = v
    }
    // 读取 VT_CERT_TECHNITIUM_TOKEN 环境变量 | 含义：Technitium API 认证 token（必填、敏感数据）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_TECHNITIUM_TOKEN")); v != "" {
        // 如果环境变量不为空，覆盖配置对象中的 Technitium.Token
        cfg.Technitium.Token = v
    }

    // === 认证相关环境变量处理 === | 用途：覆盖会话有效期

    // 读取 VT_CERT_AUTH_SESSION_TTL_HOURS 环境变量 | 含义：用户会话有效期（小时）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_AUTH_SESSION_TTL_HOURS")); v != "" {
        // 将字符串转换为整数
        n, err := strconv.Atoi(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称和期望格式
            return fmt.Errorf("环境变量 VT_CERT_AUTH_SESSION_TTL_HOURS 不是合法整数: %w", err)
        }
        // 转换成功，覆盖配置对象中的 Auth.SessionTTLHours
        cfg.Auth.SessionTTLHours = n
    }

    // === Technitium DNS 高级参数处理 === | 用途：覆盖 DNS 记录 TTL 和传播等待时间

    // 读取 VT_CERT_TECHNITIUM_DEFAULT_TTL 环境变量 | 含义：DNS TXT 记录默认 TTL（秒）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_TECHNITIUM_DEFAULT_TTL")); v != "" {
        // 将字符串转换为整数
        n, err := strconv.Atoi(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称
            return fmt.Errorf("环境变量 VT_CERT_TECHNITIUM_DEFAULT_TTL 不是合法整数: %w", err)
        }
        // 转换成功，覆盖配置对象中的 Technitium.DefaultTTL
        cfg.Technitium.DefaultTTL = n
    }

    // 读取 VT_CERT_TECHNITIUM_PROPAGATION_SEC 环境变量 | 含义：DNS 记录传播等待时间（秒）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_TECHNITIUM_PROPAGATION_SEC")); v != "" {
        // 将字符串转换为整数
        n, err := strconv.Atoi(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称
            return fmt.Errorf("环境变量 VT_CERT_TECHNITIUM_PROPAGATION_SEC 不是合法整数: %w", err)
        }
        // 转换成功，覆盖配置对象中的 Technitium.PropagationSec
        cfg.Technitium.PropagationSec = n
    }

    // === 自动续期相关环境变量处理 === | 用途：覆盖续期检查间隔、续期提前天数、是否启用续期

    // 读取 VT_CERT_AUTORENEW_CHECK_INTERVAL_HOUR 环境变量 | 含义：续期检查间隔（小时）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_AUTORENEW_CHECK_INTERVAL_HOUR")); v != "" {
        // 将字符串转换为整数
        n, err := strconv.Atoi(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称
            return fmt.Errorf("环境变量 VT_CERT_AUTORENEW_CHECK_INTERVAL_HOUR 不是合法整数: %w", err)
        }
        // 转换成功，覆盖配置对象中的 AutoRenewal.CheckIntervalHour
        cfg.AutoRenewal.CheckIntervalHour = n
    }

    // 读取 VT_CERT_AUTORENEW_RENEW_BEFORE_DAYS 环境变量 | 含义：提前多少天开始续期
    if v := strings.TrimSpace(os.Getenv("VT_CERT_AUTORENEW_RENEW_BEFORE_DAYS")); v != "" {
        // 将字符串转换为整数
        n, err := strconv.Atoi(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称
            return fmt.Errorf("环境变量 VT_CERT_AUTORENEW_RENEW_BEFORE_DAYS 不是合法整数: %w", err)
        }
        // 转换成功，覆盖配置对象中的 AutoRenewal.RenewBeforeDays
        cfg.AutoRenewal.RenewBeforeDays = n
    }

    // 读取 VT_CERT_AUTORENEW_ENABLED 环境变量 | 含义：是否启用自动续期（true/false）
    if v := strings.TrimSpace(os.Getenv("VT_CERT_AUTORENEW_ENABLED")); v != "" {
        // 将字符串转换为布尔值（支持 "true"、"1"、"yes" 等多种表示）
        b, err := strconv.ParseBool(v)
        if err != nil {
            // 如果转换失败，返回错误并说明环境变量名称
            return fmt.Errorf("环境变量 VT_CERT_AUTORENEW_ENABLED 不是合法布尔值: %w", err)
        }
        // 转换成功，覆盖配置对象中的 AutoRenewal.Enabled
        cfg.AutoRenewal.Enabled = b
    }

    // 所有环境变量处理完成，返回 nil 表示成功
    return nil
}

// validateRequired 用于校验关键配置是否存在（缺少时直接返回错误，阻止应用启动） | 输入：cfg = 配置对象 | 输出：返回错误或 nil（验证通过）
// 校验项：Technitium BaseURL、Technitium Token、ACME UserEmail（三个必填项）
func validateRequired(cfg Config) error {
    // === 校验 Technitium DNS API 必填参数 ===

    // 检查 Technitium baseUrl 是否存在且不为空
    if strings.TrimSpace(cfg.Technitium.BaseURL) == "" {
        // 如果 baseUrl 为空，返回错误并提示用户可用的解决方案（环境变量或配置文件）
        return fmt.Errorf("缺少必填配置: technitium.baseUrl（可通过环境变量 VT_CERT_TECHNITIUM_BASE_URL 提供）")
    }

    // 检查 Technitium token 是否存在且不为空
    if strings.TrimSpace(cfg.Technitium.Token) == "" {
        // 如果 token 为空，返回错误并提示用户可用的解决方案（环境变量或配置文件）
        return fmt.Errorf("缺少必填配置: technitium.token（可通过环境变量 VT_CERT_TECHNITIUM_TOKEN 提供）")
    }

    // === 校验 ACME 必填参数 ===

    // 检查 ACME userEmail 是否存在且不为空
    if strings.TrimSpace(cfg.ACME.UserEmail) == "" {
        // 如果 userEmail 为空，返回错误并提示用户可用的解决方案（环境变量或配置文件）
        return fmt.Errorf("缺少必填配置: acme.userEmail（可通过环境变量 VT_CERT_ACME_USER_EMAIL 提供）")
    }

    // 所有关键配置校验通过，返回 nil 表示验证成功
    return nil
}

// defaultConfig 用于生成应用程序的默认配置值
// 输入：无
// 输出：返回包含所有默认值的 Config 实例
// 说明：这个函数定义了所有配置参数的默认值，如果配置文件中没有指定某些参数，将使用这些默认值
func defaultConfig() Config {
    return Config{
        // === Server 配置块 === | HTTP 服务器的默认配置
        // Address: ":8080" = 监听所有网络接口的 8080 端口（用于本地开发和生产环境）
        Server: ServerConfig{Address: ":8080"},

        // === Storage 配置块 === | 数据存储的默认目录结构
        Storage: StorageConfig{
            // DataDir = "./data" = 相对路径，在应用程序运行目录下创建 data 目录
            // 用于存储所有程序生成的文件（数据库、证书、密钥等）
            DataDir:       "./data",
            // SQLitePath = "./data/app.db" = SQLite 数据库文件位置
            // 存储用户、会话、证书等数据的本地嵌入式数据库
            SQLitePath:    "./data/app.db",
            // CertsDir = "./data/certs" = SSL/TLS 证书存储目录
            // 存储通过 ACME 申请的证书文件（fullchain.pem、privkey.pem）
            CertsDir:      "./data/certs",
            // AccountKeyDir = "./data/acme" = ACME 账户私钥存储目录
            // 存储 Let's Encrypt 账户的私钥和注册信息（敏感数据）
            AccountKeyDir: "./data/acme",
        },

        // === Auth 配置块 === | 认证和会话管理的默认配置
        // SessionTTLHours = 24 = 用户会话有效期为 24 小时
        // 超过 24 小时未活动的会话将被认为过期，需要重新登录
        Auth: AuthConfig{SessionTTLHours: 24},

        // === ACME 配置块 === | Let's Encrypt 证书申请的默认配置
        // DirectoryURL = "https://acme-v02.api.letsencrypt.org/directory"
        // = Let's Encrypt 生产环境目录 URL（用于申请真实证书）
        // 生产证书有限制（每周 5 个重复域名），若测试建议改用：
        // "https://acme-staging-v02.api.letsencrypt.org/directory"（测试证书无限制）
        ACME: ACMEConfig{DirectoryURL: "https://acme-v02.api.letsencrypt.org/directory"},

        // === Technitium 配置块 === | DNS-01 验证的默认配置
        Technitium: TechnitiumConfig{
            // DefaultTTL = 120 = DNS 记录的生存时间为 120 秒
            // ACME 各验证时添加的临时 TXT 记录使用此 TTL
            // 120 秒足以完成验证且不会长期污染 DNS 缓存
            DefaultTTL:     120,
            // PropagationSec = 120 = DNS 传播等待时间为 120 秒
            // 添加 DNS 记录后，等待 120 秒使其全球传播
            // 给 DNS 解析器足够时间缓存新记录，减少 ACME 验证失败
            PropagationSec: 120,
        },

        // === AutoRenewal 配置块 === | 证书自动续期的默认配置
        AutoRenewal: AutoRenewalConfig{
            // Enabled = true = 默认启用自动续期 worker
            // 程序启动时会启动后台进程定期检查证书过期时间
            Enabled:           true,
            // CheckIntervalHour = 24 = 每 24 小时检查一次证书
            // 足够频繁地发现即将过期的证书，但不会过度消耗系统资源
            CheckIntervalHour: 24,
            // RenewBeforeDays = 30 = 证书在过期前 30 天开始续期
            // 确保在证书真正过期前（有 30 天的安全窗口）完成续期
            // 让浏览器和客户端有充足时间更新证书缓存
            RenewBeforeDays:   30,
        },
    }
}

// (c Config) SessionTTL 是 Config 的方法，用于将会话 TTL（小时）转换为 time.Duration
// 输入：无（直接使用 Config 中的 SessionTTLHours 字段）
// 输出：返回时间间隔（time.Duration），单位为纳秒
// 用途：在会话管理中使用，例如设置会话过期超时
// 示例：cfg.SessionTTL() 返回 24 小时（如果 SessionTTLHours=24）
func (c Config) SessionTTL() time.Duration {
    // 将小时值转换为 time.Duration 类型
    // 例如：24（小时）* time.Hour = 24h（86400000000000 纳秒）
    return time.Duration(c.Auth.SessionTTLHours) * time.Hour
}
