// 包：cert | 用途：ACME 证书申请和续期管理 | 关键接口/结构：Service、TechnitiumProvider
// 关键职责：1）集成 Let's Encrypt ACME 协议申请 SSL 证书 | 2）管理 ACME 账户和密钥文件 | 3）处理证书文件保存和续期 | 4）实现 DNS-01 挑战验证
package cert

// === 依赖包导入 === | 说明：ACME 证书申请和密钥管理所需的核心加密包
import (
    // crypto 包提供密码学中基本的接口 | 用途：PrivateKey 接口实现（legoUser 实现此接口）
    // | 来自：Go 标准库
    "crypto"
    // crypto/rand 包提供安全的随机数生成 | 用途：生成 ACME 账户的 RSA 2048-bit 私钥（rsa.GenerateKey）
    // | 来自：Go 标准库
    "crypto/rand"
    // crypto/rsa 包实现 RSA 公钥加密算法 | 用途：生成密钥对、密钥编码解码（ParsePKCS1PrivateKey）
    // | 来自：Go 标准库
    "crypto/rsa"
    // crypto/x509 包解析 X.509 数字证书和 PKCS#1 格式密钥 | 用途：解析 PEM 证书提取过期时间、解析 RSA 私钥
    // | 来自：Go 标准库
    "crypto/x509"
    // encoding/json 包处理 JSON 序列化 | 用途：序列化/反序列化 ACME 账户数据（account 结构体）
    // | 来自：Go 标准库
    "encoding/json"
    // encoding/pem 包处理 PEM 格式编码 | 用途：编码 RSA 私钥为 PEM 格式、解码 PEM 格式的证书和密钥
    // | 来自：Go 标准库
    "encoding/pem"
    // fmt 包提供格式化输出 | 用途：构造错误信息字符串、错误包装（fmt.Errorf）
    // | 来自：Go 标准库
    "fmt"
    // os 包提供操作系统文件操作 | 用途：读写证书文件、密钥文件、账户文件（ReadFile、WriteFile、Stat）
    // | 来自：Go 标准库
    "os"
    // path/filepath 包处理文件路径 | 用途：拼接存储路径（Join）、获取文件扩展名（Ext）
    // | 来自：Go 标准库
    "path/filepath"
    // strings 包处理字符串操作 | 用途：替换违法文件名字符（NewReplacer - 域名中的 * . / 转为 _ 等）
    // | 来自：Go 标准库
    "strings"
    // time 包处理时间操作 | 用途：获取当前时间戳（time.Now）、解析证书过期时间（cert.NotAfter）
    // | 来自：Go 标准库
    "time"
    // certificate 包（github.com/go-acme/lego）提供 ACME 证书操作 | 用途：ObtainRequest 结构体、证书申请逻辑（cli.Certificate.Obtain）
    // | 来自：第三方库 lego（Let's Encrypt ACME 客户端库）
    "github.com/go-acme/lego/v4/certificate"
    // certcrypto 包（github.com/go-acme/lego）定义证书密钥类型常量 | 用途：RSA2048 密钥类型常量
    // | 来自：第三方库 lego
    certcrypto "github.com/go-acme/lego/v4/certcrypto"
    // lego 包是 ACME 客户端主库 | 用途：NewConfig、NewClient、Certificate、Challenge、Registration 等核心 API
    // | 来自：第三方库 lego
    "github.com/go-acme/lego/v4/lego"
    // registration 包（github.com/go-acme/lego）管理 ACME 账户注册 | 用途：Register 函数、RegisterOptions 结构体、Resource 资源
    // | 来自：第三方库 lego
    "github.com/go-acme/lego/v4/registration"
    // config 包（内部）包含应用全局配置 | 用途：Config 结构体（ACME URL、存储路径）、引用 ACME、Storage 配置
    // | 来自：内部模块 vt-cert-panel/internal/config
    "vt-cert-panel/internal/config"
    // models 包（内部）定义数据结构 | 用途：Certificate 模型结构体（保存申请的证书信息）
    // | 来自：内部模块 vt-cert-panel/internal/models
    "vt-cert-panel/internal/models"
)

// Service 代表 ACME 证书管理服务
// 用途：集中管理证书申请、续期、文件保存等业务逻辑
// 持有的资源：cfg = 全局配置（ACME URL、存储路径）、provider = DNS 验证提供者（Technitium）
type Service struct {
    // cfg 表示全局应用配置对象 | 类型：*config.Config | 作用域：内部使用
    // 用于获取 ACME 目录 URL、证书存储路径、账户文件存储路径等配置
    cfg *config.Config
    // provider 表示 DNS-01 验证提供者实例 | 类型：*TechnitiumProvider | 作用域：内部使用
    // 实现 Technitium DNS API 调用，在 ACME 挑战验证期间用于添加/删除 TXT 记录
    provider *TechnitiumProvider
}

// account 代表 ACME 账户信息，用于与 Let's Encrypt 交互
// 用途：保存 ACME 账户的邮箱、注册资源、RSA 私钥等信息
// 注意：此结构体通常以 JSON 格式存储在 account.json 文件中（⚠️ 敏感数据）
type account struct {
    // Email 表示与 ACME 账户关联的邮箱地址 | 类型：string | 作用域：ACME 通知、恢复用
    // 用户无法从此邮箱修改账户，主要用于接收证书过期提醒
    Email string `json:"email"`
    // Registration 表示 Let's Encrypt 返回的注册资源（账户 URL 等） | 类型：*registration.Resource | 作用域：ACME 通信
    // 包含账户在 Let's Encrypt 服务器上的唯一标识和状态信息
    // omitempty：如果为 nil 则不序列化到 JSON（新账户注册时为 nil，注册后才有值）
    Registration *registration.Resource `json:"registration,omitempty"`
    // KeyPEM 表示 ACME 账户的私钥（PEM 格式字符串） | 类型：string | 作用域：密钥管理
    // ⚠️ 敏感数据：这是账户的唯一识别凭证，丢失或泄露会导致任何人都能通过此账户申请证书
    // 存储时必须限制文件权限为 0o600（仅所有者可读写）
    KeyPEM string `json:"keyPem"`
}

// NewService 是 Service 的构造函数，创建证书管理服务实例
// 输入：cfg = 全局应用配置、provider = DNS 验证提供者（Technitium）
// 输出：返回初始化的 *Service 实例
// 说明：简单的依赖注入，将配置和服务引用保存到 Service 中供后续使用
func NewService(cfg *config.Config, provider *TechnitiumProvider) *Service {
    // 创建并返回 Service 实例，关联配置和 DNS 提供者
    return &Service{cfg: cfg, provider: provider}
}

// (s *Service) Issue 用于通过 ACME 申请新的 SSL 证书
// 输入：domains = 域名列表（如 ["example.com", "www.example.com"]）、email = 申请人邮箱
// 输出：返回申请成功的证书信息（*models.Certificate）或错误
// 说明：执行完整的 ACME 申请流程（验证 DNS、申请证书、保存文件）
func (s *Service) Issue(domains []string, email string) (*models.Certificate, error) {
    // 验证域名列表不为空
    if len(domains) == 0 {
        // 如果没有提供任何域名，直接返回错误
        return nil, fmt.Errorf("域名不能为空")
    }

    // 创建 ACME 客户端和账户对象
    // newClient() 会加载或创建 ACME 账户，初始化 lego 客户端和 DNS-01 验证器
    cli, acc, err := s.newClient(email)
    if err != nil {
        // 如果客户端创建失败（账户加载错误、密钥解析错误等），返回错误
        return nil, err
    }
    // 忽略 acc 变量，因为在此函数中不需要用到它
    _ = acc

    // 构造 ACME 证书申请请求
    // ObtainRequest 指定要申请的域名列表和是否需要完整证书链（Bundle=true）
    request := certificate.ObtainRequest{Domains: domains, Bundle: true}
    // 调用 lego 客户端申请证书
    // 此步骤会：
    // 1. 向 Let's Encrypt 服务器提交申请
    // 2. Let's Encrypt 返回 DNS-01 挑战任务（需要在 DNS 添加特定 TXT 记录）
    // 3. 调用 DNS-01 provider（TechnitiumProvider）添加 TXT 记录
    // 4. Let's Encrypt 验证 DNS 记录，验证通过后签发证书
    result, err := cli.Certificate.Obtain(request)
    if err != nil {
        // 如果申请失败（DNS 验证失败、ACME 错误等），返回错误
        return nil, fmt.Errorf("证书申请失败: %w", err)
    }

    // 创建证书存储目录
    // 使用主域名（domains[0]）的清理版本作为目录名
    // 例如：*.example.com → 存储目录为 wildcard_example_com
    certDir := filepath.Join(s.cfg.Storage.CertsDir, sanitizeName(domains[0]))
    // 创建目录，如果已存在则不报错；0o755 表示权限（所有者读写执行，其他人只读执行）
    if err := os.MkdirAll(certDir, 0o755); err != nil {
        // 如果目录创建失败（权限问题、磁盘满等），返回错误
        return nil, err
    }

    // 保存证书链文件（fullchain.pem）
    // fullchain.pem 包含完整的证书链：叶子证书 + 中间证书
    // 这个文件用于 Web 服务器配置（如 Nginx 的 ssl_certificate）
    fullchain := filepath.Join(certDir, "fullchain.pem")
    // 写入文件，0o644 表示权限（所有者读写，其他人只读）
    if err := os.WriteFile(fullchain, result.Certificate, 0o644); err != nil {
        // 如果文件写入失败，返回错误
        return nil, err
    }

    // 保存私钥文件（privkey.pem）
    // privkey.pem 是账户的 RSA 私钥，用于 HTTPS 加密通信
    // ⚠️ 敏感数据：必须妥善保护，只有 Web 服务器应有读取权限
    privkey := filepath.Join(certDir, "privkey.pem")
    // 写入文件，0o600 表示权限（只有所有者可读写，其他人完全无权限）⚠️ 安全：最严格的权限设置
    if err := os.WriteFile(privkey, result.PrivateKey, 0o600); err != nil {
        // 如果文件写入失败，返回错误
        return nil, err
    }

    // 解析证书的过期时间
    // parseExpires() 从 PEM 格式的证书中提取 X.509 证书对象，获取 NotAfter 字段
    // 返回的是 UTC 时间，用于后续的续期检查
    expires, _ := parseExpires(result.Certificate)

    // 构造并返回证书信息对象
    // 包含证书的所有元数据信息（域名、邮箱、文件路径、签发时间等）
    return &models.Certificate{
        // 主域名（通常是域名列表的第一个）
        PrimaryDomain: domains[0],
        // 申请的所有域名列表
        Domains: domains,
        // 申请人邮箱
        Email: email,
        // 证书文件存储目录
        CertDir: certDir,
        // 完整证书链文件路径
        FullchainPath: fullchain,
        // 私钥文件路径
        PrivkeyPath: privkey,
        // 证书过期时间
        ExpiresAt: expires,
        // 证书状态
        Status: "issued",
        // 创建时间戳
        CreatedAt: time.Now().UTC(),
        // 最后更新时间戳
        UpdatedAt: time.Now().UTC(),
    }, nil
}

// (s *Service) Renew 用于续期（更新）已有的 SSL 证书
// 输入：existing = 现有证书对象（包含原始申请信息：域名、邮箱等）
// 输出：返回更新后的证书对象（新的文件路径、过期时间等）或错误
// 说明：这是一个便利方法，实际上重新申请证书然后更新数据库记录
func (s *Service) Renew(existing *models.Certificate) (*models.Certificate, error) {
    // 调用 Issue() 重新申请证书
    // 使用相同的域名列表和邮箱，这样可以获取新的证书文件和过期时间
    newCert, err := s.Issue(existing.Domains, existing.Email)
    if err != nil {
        // 如果申请失败，直接返回错误（不修改现有 existing 对象）
        return nil, err
    }

    // 更新现有证书对象的字段为新申请的证书数据
    // 这里采用原地修改策略，保留原有证书的创建时间和 ID（如果有）

    // 更新证书存储目录
    existing.CertDir = newCert.CertDir
    // 更新证书链文件路径
    existing.FullchainPath = newCert.FullchainPath
    // 更新私钥文件路径
    existing.PrivkeyPath = newCert.PrivkeyPath
    // 更新过期时间为新证书的过期时间
    existing.ExpiresAt = newCert.ExpiresAt
    // 更新状态为 "renewed"（已续期）
    existing.Status = "renewed"
    // 清空上次出错信息（因为本次续期成功）
    existing.LastError = ""
    // 记录最后一次续期时间
    existing.LastRenewedAt = time.Now().UTC()
    // 更新最后修改时间戳
    existing.UpdatedAt = time.Now().UTC()

    // 返回更新后的证书对象
    return existing, nil
}

// (s *Service) newClient 用于创建和初始化 ACME 客户端
// 输入：email = 申请人邮箱地址
// 输出：返回初始化的 lego 客户端、ACME 账户对象和可能的错误
// 说明：这是一个复杂的初始化过程，涉及账户加载、密钥解析、ACME 注册等步骤
func (s *Service) newClient(email string) (*lego.Client, *account, error) {
    // 加载现有 ACME 账户或创建新账户
    // loadOrCreateAccount() 会：
    // 1. 检查 account.json 是否存在
    // 2. 如果存在，读取和解析账户数据
    // 3. 如果不存在，生成新的 2048-bit RSA 密钥对并保存
    acc, err := s.loadOrCreateAccount(email)
    if err != nil {
        // 如果账户加载/创建失败，返回错误
        return nil, nil, err
    }

    // 解析 PEM 格式的私钥字符串为 PEM 块
    // PEM 格式是密钥的文本编码方式，需要先解码成二进制数据
    privateKeyBlock, _ := pem.Decode([]byte(acc.KeyPEM))
    if privateKeyBlock == nil {
        // 如果 PEM 解码失败（格式错误），返回错误
        return nil, nil, fmt.Errorf("ACME账户密钥解析失败")
    }

    // 将 PEM 块中的二进制数据解析为 RSA 私钥对象
    // ParsePKCS1PrivateKey 适用于 "RSA PRIVATE KEY" 格式的密钥
    pk, err := x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
    if err != nil {
        // 如果密钥解析失败（格式错误或损坏），返回错误
        return nil, nil, err
    }

    // 创建 legoUser 对象
    // legoUser 实现了 lego 库所需的 User 接口（GetEmail、GetPrivateKey、GetRegistration）
    legoUser := &legoUser{email: acc.Email, key: pk, registration: acc.Registration}

    // 创建 lego 客户端配置对象
    // 此配置包含用户信息和 ACME 服务器 URL 等参数
    cfg := lego.NewConfig(legoUser)
    // 设置 ACME 服务器目录 URL（从全局配置读取，可能是 Let's Encrypt 生产环境或测试环境）
    // 生产：https://acme-v02.api.letsencrypt.org/directory
    // 测试：https://acme-staging-v02.api.letsencrypt.org/directory
    cfg.CADirURL = s.cfg.ACME.DirectoryURL
    // 设置密钥类型为 RSA 2048-bit（目前是 Let's Encrypt 的推荐标准）
    // ⚠️ 注意：以后可能升级为 RSA 4096 或 ECDP256 以增强安全性
    cfg.Certificate.KeyType = certcrypto.RSA2048

    // 使用配置创建 lego 客户端
    // lego 客户端负责与 Let's Encrypt ACME 服务器通信
    client, err := lego.NewClient(cfg)
    if err != nil {
        // 如果客户端创建失败，返回错误
        return nil, nil, err
    }

    // 设置 DNS-01 验证提供者
    // DNS-01 提供者负责在 ACME 挑战验证时添加/删除 DNS TXT 记录
    // 我们使用 TechnitiumProvider（Technitium DNS API 实现）
    if err := client.Challenge.SetDNS01Provider(s.provider); err != nil {
        // 如果设置失败（提供者初始化错误），返回错误
        return nil, nil, err
    }

    // 检查账户是否已在 Let's Encrypt 注册过
    // 如果 legoUser.registration 为 nil，说明这是新账户，需要注册
    if legoUser.registration == nil {
        // 向 Let's Encrypt 注册新账户
        // Register 会调用 ACME 服务器的 newAccount 接口，返回账户资源和 URL
        reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
        if err != nil {
            // 如果注册失败（网络错误、服务器拒绝等），返回错误
            return nil, nil, err
        }

        // 更新本地账户对象的注册信息
        // 这样下次就不需要重新注册了
        legoUser.registration = reg
        acc.Registration = reg
        // 保存更新后的账户信息到文件
        // 下次启动时直接读取此文件，不需要重新注册
        if err := s.saveAccount(acc); err != nil {
            // 如果保存失败，返回错误
            return nil, nil, err
        }
    }

    // 返回初始化完成的 lego 客户端、账户对象和 nil 错误
    return client, acc, nil
}
// (s *Service) loadOrCreateAccount 用于加载现有 ACME 账户或创建新账户
// 输入：email = 申请人邮箱地址
// 输出：返回 ACME 账户对象或错误
// 说明：实现了账户数据的持久化存储，避免重复生成密钥对
func (s *Service) loadOrCreateAccount(email string) (*account, error) {
    // 构造账户文件路径
    // 账户信息保存在 storage.account_key_dir 目录下的 account.json 文件中
    path := filepath.Join(s.cfg.Storage.AccountKeyDir, "account.json")

    // 检查账户文件是否存在
    // Stat 返回文件信息，如果文件不存在则 err != nil
    if _, err := os.Stat(path); err == nil {
        // 文件存在，读取并解析现有账户数据

        // 读取文件全部内容
        content, err := os.ReadFile(path)
        if err != nil {
            // 如果读取失败（权限问题等），返回错误
            return nil, err
        }

        // 将 JSON 字节数据解析为账户对象
        acc := &account{}
        if err := json.Unmarshal(content, acc); err != nil {
            // 如果 JSON 解析失败（格式错误或损坏），返回错误
            return nil, err
        }

        // 如果文件中的邮箱为空，使用传入的邮箱值
        // 这样可以处理某些特殊情况（如账户文件部分损坏但仍可恢复）
        if acc.Email == "" {
            acc.Email = email
        }

        // 返回从文件加载的账户对象
        return acc, nil
    }

    // 账户文件不存在，创建新账户
    // 生成 2048-bit RSA 密钥对
    // 这是账户的唯一标识，必须安全保存
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        // 如果密钥生成失败（极罕见，系统资源不足或随机数生成器错误），返回错误
        return nil, err
    }

    // 将 RSA 私钥编码为 PEM 格式字符串
    // x509.MarshalPKCS1PrivateKey 将密钥对象转换为 PKCS#1 格式的字节
    // pem.EncodeToMemory 将其编码为 PEM 格式（包含头尾标识和 Base64 编码）
    pemBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

    // 创建新账户对象
    // 注册时没有 Registration 信息（设为 nil），注册成功后会填充
    acc := &account{Email: email, KeyPEM: string(pemBlock)}

    // 保存新账户到文件
    // saveAccount 会将账户以 JSON 格式写入磁盘，权限为 0o600（仅所有者可读写）
    if err := s.saveAccount(acc); err != nil {
        // 如果保存失败（磁盘满、权限问题等），返回错误
        return nil, err
    }

    // 返回新创建的账户对象
    return acc, nil
}

// (s *Service) saveAccount 用于将 ACME 账户信息保存到文件
// 输入：acc = 要保存的账户对象
// 输出：返回错误或 nil（保存成功）
// 说明：将账户数据序列化为 JSON 格式并写入磁盘，使用最严格的文件权限（0o600）保护
func (s *Service) saveAccount(acc *account) error {
    // 将账户对象序列化为格式化的 JSON 字节数据
    // MarshalIndent 会产生缩进的、可读的 JSON 格式（缩进为两个空格）
    // 这样即使直接编辑文件也比较方便（虽然不建议手动编辑敏感文件）
    content, err := json.MarshalIndent(acc, "", "  ")
    if err != nil {
        // 如果序列化失败（理论上不应该发生，除非有内部 bug），返回错误
        return err
    }

    // 将 JSON 数据写入文件
    // 文件路径：{account_key_dir}/account.json
    // 权限：0o600 表示仅所有者可读写，其他用户无任何权限（最严格的安全设置）
    // ⚠️ 安全：此文件包含 ACME 账户的私钥，必须保护好，不能让其他用户访问
    return os.WriteFile(filepath.Join(s.cfg.Storage.AccountKeyDir, "account.json"), content, 0o600)
}

// parseExpires 用于解析 PEM 格式的 X.509 证书并提取过期时间
// 输入：pemBytes = PEM 格式的证书字节数据（如 fullchain.pem 的二进制内容）
// 输出：返回证书的过期时间（time.Time）和可能的错误
// 说明：用于从签发的证书中提取元数据，尤其是续期检查需要用到过期时间
func parseExpires(pemBytes []byte) (time.Time, error) {
    // 从 PEM 格式的字节数据中解码第一个 PEM 块
    // PEM 文件以 -----BEGIN CERTIFICATE----- 开头，-----END CERTIFICATE----- 结尾
    // Decode 返回 PEM 块和剩余的字节数据（如果有多个块）
    block, _ := pem.Decode(pemBytes)
    if block == nil {
        // 如果没有找到有效的 PEM 块（格式错误），返回错误
        return time.Time{}, fmt.Errorf("证书PEM解析失败")
    }

    // 将 PEM 块中的 DER 编码数据解析为 X.509 证书对象
    // X.509 是数字证书的国际标准格式，包含公钥、身份信息、有效期等
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        // 如果证书数据损坏或不符合 X.509 规范，返回错误
        return time.Time{}, err
    }

    // 返回证书的过期时间（NotAfter 字段）
    // 转换为 UTC 时区以保证一致性（避免时区相关的 bug）
    return cert.NotAfter.UTC(), nil
}

// sanitizeName 用于清理域名字符串，将其转换为合法的文件系统目录名
// 输入：domain = 域名字符串（可能包含 * / . 等特殊字符）
// 输出：返回清理后的字符串，可安全用作目录名
// 说明：用于将证书存储目录名从域名生成，避免文件系统路径冲突和特殊字符问题
func sanitizeName(domain string) string {
    // 创建字符串替换器
    // 将以下特殊字符替换为安全字符：
    // * → wildcard（泛域名标记，如 *.example.com → wildcard_example_com）
    // . → _ （点号替换为下划线，避免目录名中出现多个点）
    // / → _ （斜杠替换为下划线，避免被解释为路径分隔符）
    replacer := strings.NewReplacer("*", "wildcard", ".", "_", "/", "_")

    // 应用替换规则
    // 例如：
    // - example.com → example_com
    // - www.example.com → www_example_com
    // - *.example.com → wildcard_example_com
    // - sub.*.example.com → sub_wildcard_example_com（虽然这种格式很罕见）
    return replacer.Replace(domain)
}

// legoUser 实现了 lego 库所需的 User 接口
// 用途：为 ACME 客户端提供账户信息（邮箱、密钥、注册资源）
// 注意：此类型实现了以下接口方法：GetEmail、GetRegistration、GetPrivateKey
type legoUser struct {
    // email 表示 ACME 账户的邮箱地址 | 类型：string | 作用域：ACME 通知
    // 用于显示和接收 Let's Encrypt 的通知和过期提醒
    email string
    // key 表示 ACME 账户的 RSA 私钥对象 | 类型：*rsa.PrivateKey | 作用域：加密通信
    // 用于对 ACME 请求进行签名，证明账户身份
    // ⚠️ 敏感数据：此私钥必须绝对保密，丢失或泄露会导致任何人都能以此账户申请证书
    key *rsa.PrivateKey
    // registration 表示 Let's Encrypt 返回的账户注册信息 | 类型：*registration.Resource | 作用域：ACME 通信
    // 包含账户在服务器上的唯一 URL 和状态，用于后续与 Let's Encrypt 的交互
    registration *registration.Resource
}

// (u *legoUser) GetEmail 返回账户邮箱地址
// 用途：lego 客户端调用此方法获取账户邮箱
// 实现了 lego.User 接口的 GetEmail 方法
func (u *legoUser) GetEmail() string {
    // 直接返回 legoUser 中存储的邮箱字符串
    return u.email
}

// (u *legoUser) GetRegistration 返回账户注册资源
// 用途：lego 客户端调用此方法获取账户在 Let's Encrypt 服务器上的注册信息
// 实现了 lego.User 接口的 GetRegistration 方法
func (u *legoUser) GetRegistration() *registration.Resource {
    // 直接返回 legoUser 中存储的注册资源
    // 如果是新账户且未注册过，此值为 nil（后续会由 lego 客户端更新）
    return u.registration
}

// (u *legoUser) GetPrivateKey 返回账户的私钥对象
// 用途：lego 客户端调用此方法获取私钥用于签署 ACME 请求
// 实现了 lego.User 接口的 GetPrivateKey 方法
// 返回值：crypto.PrivateKey 接口（在此实现中是 *rsa.PrivateKey）
func (u *legoUser) GetPrivateKey() crypto.PrivateKey {
    // 直接返回 legoUser 中存储的 RSA 私钥
    // ⚠️ 安全警告：返回的是指针，调用者不应修改此私钥（虽然 Go 无法完全防止）
    return u.key
}
