// 包：repository | 用途：数据访问层（DAO）- 负责所有数据库操作 | 关键接口/结构：CertificateRepository、UserRepository、SessionRepository
// 关键职责：1）将数据模型持久化到数据库（CRUD 操作）| 2）提供数据查询接口 | 3）处理数据库类型与 Go 类型的转换
package repository

// === 依赖包导入 === | 说明：数据库操作和数据序列化所需的核心包
import (
    // database/sql 包提供通用的数据库操作接口 | 用途：预处理 SQL 查询、执行插入/更新/删除、响应扫描
    // | 来自：Go 标准库
    "database/sql"
    // encoding/json 包处理 JSON 序列化和反序列化 | 用途：将域名数组序列化为 JSON 字符串存储在数据库、反序列化读出
    // | 来自：Go 标准库
    "encoding/json"
    // time 包处理时间操作 | 用途：时间戳转换（time.RFC3339 格式）、时间解析和比较
    // | 来自：Go 标准库
    "time"
    // models 包（内部）定义数据结构 | 用途：Certificate 模型结构体（对应数据库 certificates 表）
    // | 来自：内部模块 vt-cert-panel/internal/models
    "vt-cert-panel/internal/models"
)

// CertificateRepository 是证书数据访问对象（DAO）
// 用途：管理证书数据的数据库读写操作，实现 CRUD 完整接口
// 持有的资源：db = 数据库连接池
type CertificateRepository struct {
    // db 表示数据库连接池对象 | 类型：*sql.DB | 作用域：所有 SQL 操作
    // 由应用启动时创建，所有数据库操作都通过此连接池进行
    db *sql.DB
}

// NewCertificateRepository 是 CertificateRepository 的构造函数
// 输入：db = 数据库连接池
// 输出：返回初始化的 *CertificateRepository 实例
// 说明：简单的依赖注入，将数据库连接池关联到仓库中供后续操作使用
func NewCertificateRepository(db *sql.DB) *CertificateRepository {
    // 创建并返回 CertificateRepository 实例，关联数据库连接
    return &CertificateRepository{db: db}
}

// (r *CertificateRepository) Create 用于创建新的证书记录
// 输入：cert = 证书对象（包含所有证书信息）
// 输出：返回新插入记录的 ID 和可能的错误
// 说明：执行 INSERT 操作，将证书信息保存到数据库
func (r *CertificateRepository) Create(cert *models.Certificate) (int64, error) {
    // 将域名数组序列化为 JSON 格式
    // 这样可以将多个域名存储在单一的数据库字段中
    // 例如：["example.com", "www.example.com"] → "[\"example.com\",\"www.example.com\"]"
    domains, _ := json.Marshal(cert.Domains)
    // 获取当前时间戳（RFC3339 格式），用于 created_at 和 updated_at 字段
    // RFC3339 是国际标准时间格式，适合数据库存储和跨系统交互
    now := time.Now().UTC().Format(time.RFC3339)
    // 执行 INSERT SQL 语句，将证书数据插入 certificates 表
    // 使用参数化查询（?占位符）防止 SQL 注入
    result, err := r.db.Exec(`INSERT INTO certificates(primary_domain, domains, email, cert_dir, fullchain_path, privkey_path, expires_at, status, last_error, last_renewed_at, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, cert.PrimaryDomain, string(domains), cert.Email, cert.CertDir, cert.FullchainPath, cert.PrivkeyPath, cert.ExpiresAt.UTC().Format(time.RFC3339), cert.Status, cert.LastError, cert.LastRenewedAt.UTC().Format(time.RFC3339), now, now)
    if err != nil {
        // 如果执行失败（语法错误、约束冲突等），返回错误
        return 0, err
    }
    // 获取新插入记录的自动递增 ID
    // LastInsertId() 返回 INSERT 操作后数据库自动生成的主键值
    return result.LastInsertId()
}

// (r *CertificateRepository) Update 用于更新现有的证书记录
// 输入：cert = 证书对象（必须包含有效的 ID，其他字段为新值）
// 输出：返回错误或 nil（更新成功）
// 说明：执行 UPDATE 操作，更新指定 ID 的证书记录
func (r *CertificateRepository) Update(cert *models.Certificate) error {
    // 将新域名数组序列化为 JSON 格式
    // 与 Create 方法相同，用于存储多个域名
    domains, _ := json.Marshal(cert.Domains)
    // 执行 UPDATE SQL 语句，根据证书 ID 更新指定记录
    // 更新的字段包括：domains、email、cert_dir、fullchain_path、privkey_path、expires_at、status、last_error、last_renewed_at、updated_at
    // 注意：created_at 不更新（插入时确定，永不改变）
    _, err := r.db.Exec(`UPDATE certificates SET domains=?, email=?, cert_dir=?, fullchain_path=?, privkey_path=?, expires_at=?, status=?, last_error=?, last_renewed_at=?, updated_at=? WHERE id=?`,
        string(domains), cert.Email, cert.CertDir, cert.FullchainPath, cert.PrivkeyPath, cert.ExpiresAt.UTC().Format(time.RFC3339), cert.Status, cert.LastError, cert.LastRenewedAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), cert.ID)
    // 直接返回执行结果的错误值（如果有的话）
    return err
}

// (r *CertificateRepository) List 用于查询所有证书记录
// 输入：无
// 输出：返回证书列表和可能的错误
// 说明：返回所有证书，按 ID 倒序排列（最新的优先）
func (r *CertificateRepository) List() ([]models.Certificate, error) {
    // 执行 SELECT SQL 查询，获取所有证书记录
    // COALESCE(last_error,'') 如果 last_error 为 NULL 则返回空字符串（避免 NULL 值扫描问题）
    // COALESCE(last_renewed_at,'') 同样处理 last_renewed_at 字段（nullable 字段）
    // ORDER BY id DESC 按 ID 倒序排列（新增的证书优先显示）
    rows, err := r.db.Query(`SELECT id, primary_domain, domains, email, cert_dir, fullchain_path, privkey_path, expires_at, status, COALESCE(last_error,''), COALESCE(last_renewed_at,''), created_at, updated_at FROM certificates ORDER BY id DESC`)
    if err != nil {
        // 如果查询执行失败，返回错误
        return nil, err
    }

    // 确保查询结果集在函数返回前被关闭（释放数据库资源）
    defer rows.Close()

    // 创建一个空的证书列表（容量为 0，但可以动态增长）
    result := make([]models.Certificate, 0)
    // 逐行扫描查询结果
    for rows.Next() {
        // 调用 scanCertificate 辅助函数扫描一行数据
        // scanCertificate 负责数据库类型到 Go 类型的转换（如 JSON 反序列化、时间格式解析）
        item, err := scanCertificate(rows)
        if err != nil {
            // 如果扫描失败，停止并返回错误
            return nil, err
        }
        // 将扫描的证书对象添加到结果列表中
        result = append(result, *item)
    }

    // 返回所有证书列表
    return result, nil
}

// (r *CertificateRepository) GetByID 用于根据 ID 查询单个证书
// 输入：id = 证书的主键 ID
// 输出：返回证书对象或错误（如果记录不存在则返回 sql.ErrNoRows）
// 说明：通过主键快速查询，返回单个证书记录
func (r *CertificateRepository) GetByID(id int64) (*models.Certificate, error) {
    // 执行 SELECT SQL 查询，根据 ID 查询单条记录
    // QueryRow 与 Query 不同，用于查询单行记录（不返回行集合）
    // COALESCE 处理 nullable 字段，确保不会出现 NULL 扫描错误
    row := r.db.QueryRow(`SELECT id, primary_domain, domains, email, cert_dir, fullchain_path, privkey_path, expires_at, status, COALESCE(last_error,''), COALESCE(last_renewed_at,''), created_at, updated_at FROM certificates WHERE id=?`, id)
    // 直接调用 scanCertificate 扫描和解析查询结果
    // 如果记录不存在，scanCertificate 会返回 sql.ErrNoRows 错误
    return scanCertificate(row)
}

// (r *CertificateRepository) FindDueForRenew 用于查询需要续期的证书
// 输入：before = 时间截止点（所有 expires_at <= before 的证书视为需要续期）
// 输出：返回需要续期的证书列表和可能的错误
// 说明：用于自动续期服务定期检查和续期将过期的证书
func (r *CertificateRepository) FindDueForRenew(before time.Time) ([]models.Certificate, error) {
    // 执行 SELECT SQL 查询，查找所有过期时间小于等于指定时间的证书
    // before.UTC().Format(time.RFC3339) 将 Go 时间对象转换为 RFC3339 字符串格式以匹配数据库存储格式
    // 这样可以正确比较时间大小（例如：过期时间 30 天内的证书）
    rows, err := r.db.Query(`SELECT id, primary_domain, domains, email, cert_dir, fullchain_path, privkey_path, expires_at, status, COALESCE(last_error,''), COALESCE(last_renewed_at,''), created_at, updated_at FROM certificates WHERE expires_at <= ?`, before.UTC().Format(time.RFC3339))
    if err != nil {
        // 如果查询执行失败，返回错误
        return nil, err
    }

    // 确保查询结果集在函数返回前被关闭
    defer rows.Close()

    // 创建一个空的证书列表
    result := make([]models.Certificate, 0)
    // 逐行扫描查询结果
    for rows.Next() {
        // 扫描一行数据并转换为证书对象
        item, err := scanCertificate(rows)
        if err != nil {
            // 如果扫描失败，停止并返回错误
            return nil, err
        }
        // 将证书添加到结果列表
        result = append(result, *item)
    }

    // 返回需要续期的证书列表
    return result, nil
}

// scanner 是一个接口，表示任何可以扫描数据库行数据的对象
// 用途：使 scanCertificate 函数既可以扫描 sql.Rows，也可以扫描 sql.Row
// 这是一个常见的 Go 设计模式，通过接口增加代码灵活性
type scanner interface {
    // Scan 方法从数据库行扫描数据到目标变量
    // dest 是可变参数列表，每个参数对应数据库的一列
    Scan(dest ...any) error
}

// scanCertificate 是辅助函数，负责将数据库行数据转换为证书对象
// 输入：s = scanner 接口实现者（可以是 *sql.Row 或 *sql.Rows）
// 输出：返回转换后的证书对象和可能的错误
// 说明：处理数据库类型到 Go 类型的转换，特别是复杂类型（JSON、时间格式）
func scanCertificate(s scanner) (*models.Certificate, error) {
    // 创建证书对象容器
    var item models.Certificate
    // 从数据库读出的 domains 是 JSON 字符串，需要声明临时变量存储
    var domainsJSON string
    // 从数据库读出的时间字段都是 RFC3339 格式字符串，需要声明临时变量存储
    // 然后再解析为 time.Time 对象
    var expiresAt, renewedAt, createdAt, updatedAt string

    // 从数据库行扫描所有 13 个列的数据到对应的变量中
    // SQL 查询的列顺序：id, primary_domain, domains, email, cert_dir, fullchain_path, privkey_path, expires_at, status, last_error, last_renewed_at, created_at, updated_at
    if err := s.Scan(&item.ID, &item.PrimaryDomain, &domainsJSON, &item.Email, &item.CertDir, &item.FullchainPath, &item.PrivkeyPath, &expiresAt, &item.Status, &item.LastError, &renewedAt, &createdAt, &updatedAt); err != nil {
        // 如果扫描失败（类型不匹配、列数不对等），返回错误
        return nil, err
    }

    // 解析 domains JSON 字符串为字符串切片
    // 例如："[\"example.com\",\"www.example.com\"]" → ["example.com", "www.example.com"]
    // 使用 _ 忽略错误，因为 domains 不为空时格式总是正确的
    _ = json.Unmarshal([]byte(domainsJSON), &item.Domains)
    // 解析 expiresAt 时间字符串为 time.Time 对象
    // RFC3339 格式例如："2024-12-31T23:59:59Z"
    // 使用 _ 忽略错误（虽然理论上不应该发生，因为数据库中的格式总是正确的）
    item.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
    // 如果 renewedAt 不为空，才进行解析（因为这是一个 nullable 字段）
    if renewedAt != "" {
        item.LastRenewedAt, _ = time.Parse(time.RFC3339, renewedAt)
    }

    // 解析 createdAt 时间字符串
    item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    // 解析 updatedAt 时间字符串
    item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

    // 返回完全转换和初始化的证书对象
    return &item, nil
}
