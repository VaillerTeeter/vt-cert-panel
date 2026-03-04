// 包：db | 用途：数据库初始化与连接管理 | 关键函数：Open、migrate
// 关键职责：1）打开 SQLite 数据库连接 | 2）配置连接池参数 | 3）执行数据库迁移（创建表） | 4）管理数据库生命周期
package db

// === 依赖包导入 === | 说明：数据库操作和连接管理所需的核心包
import (
    // database/sql 包提供通用的 SQL 数据库接口 | 用途：打开数据库连接（sql.Open）、执行 SQL 语句（db.Exec）
    // | 来自：Go 标准库
    "database/sql"
    // fmt 包提供格式化输出功能 | 用途：错误消息格式化（fmt.Errorf）
    // | 来自：Go 标准库
    "fmt"
    // time 包处理时间操作 | 用途：设置连接超时（time.Minute）
    // | 来自：Go 标准库
    "time"
    // sqlite 驱动包（使用匿名导入 _）| 用途：注册 SQLite 驱动到 database/sql | 特点：纯 Go 实现，无 C 依赖
    // 匿名导入（_）意味着只执行 init 函数进行驱动注册，不直接引用包中的符号
    // | 来自：modernc.org/sqlite 外部包
    _ "modernc.org/sqlite"
)

// Open 用于打开 SQLite 数据库连接并配置连接池
// 输入：sqlitePath = SQLite 数据库文件的路径（如 ./data/app.db）
// 输出：返回配置完初的 *sql.DB 实例或错误
// 可能失败：1）数据库文件无法打开 | 2）迁移（表创建）失败
// 执行流程：打开连接 -> 配置连接池 -> 执行数据库迁移 -> 返回 DB 实例
func Open(sqlitePath string) (*sql.DB, error) {
    // 打开 SQLite 数据库连接
    // 第一个参数 "sqlite" 是驱动名称（由 modernc.org/sqlite 驱动注册）
    // 第二个参数是数据库文件路径，若文件不存在会自动创建
    db, err := sql.Open("sqlite", sqlitePath)
    if err != nil {
        // 如果打开失败，返回错误（通常是权限问题或磁盘空间不足）
        return nil, fmt.Errorf("打开SQLite失败: %w", err)
    }

    // === 配置连接池参数（SQLite 特定优化）===
    // SetConnMaxLifetime: 单个连接的最大生存时间
    // 设置为 30 分钟，防止长期空闲连接占用资源
    // SQLite 通常使用单个文件，长生存时间的连接可能导致文件锁定问题
    db.SetConnMaxLifetime(30 * time.Minute)

    // SetMaxOpenConns: 最大开放连接数
    // 设置为 1，因为 SQLite 是单线程数据库，同时只能处理一个写操作
    // 多个并发连接会导致 "database is locked" 错误
    // 使用 1 个连接时，database/sql 会自动处理并发请求的序列化
    db.SetMaxOpenConns(1)

    // SetMaxIdleConns: 最大空闲连接数
    // 设置为 1，与最大开放连接数保持一致
    // 空闲连接会定期超时释放（由 SetConnMaxLifetime 控制）
    db.SetMaxIdleConns(1)

    // 执行数据库迁移：创建必要的表（users、sessions、certificates）
    if err := migrate(db); err != nil {
        // 如果迁移失败，返回错误并关闭数据库连接
        return nil, err
    }

    // 返回已初始化的数据库实例，可用于后续数据操作
    return db, nil
}

// migrate 是一个内部函数，用于执行数据库迁移
// 输入：db = 已打开的数据库连接
// 输出：返回错误或 nil（迁移成功）
// 可能失败：1）SQL 语法错误 | 2）表已存在但定义冲突
// 用途：初始化数据库表结构，在首次运行时创建必要的表
// 说明：使用 CREATE TABLE IF NOT EXISTS 语句，已有表时跳过创建
func migrate(db *sql.DB) error {
    // 定义所有需要创建的表的 SQL 语句
    // 这些语句会按顺序执行，表已存在时不会重复创建
    queries := []string{
        // === users 表 === | 用途：存储应用程序用户
        // 表结构说明：
        // - id: 用户唯一标识（自增主键）| 数据库自动生成 1, 2, 3 ...
        // - username: 登录用户名 | 类型：TEXT | 约束：不为空、全局唯一（prevent duplicate users）
        // - password_hash: bcrypt 哈希密码 | 类型：TEXT | 约束：不为空、永不存储明文密码
        // - created_at: 用户创建时间 | 类型：TEXT | 格式：RFC3339（如 2026-03-02T12:00:00Z）
        `CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            created_at TEXT NOT NULL
        );`,

        // === sessions 表 === | 用途：存储用户会话信息（登录状态）
        // 表结构说明：
        // - id: 会话唯一标识 | 类型：TEXT | 值：UUID 或随机字符串（如 a1b2c3...）| 作用：存储在 Cookie 中
        // - user_id: 关联的用户 ID | 类型：INTEGER | 用于查询当前登录用户 | 约束：不为空、外键引用 users(id)
        // - expires_at: 会话过期时间 | 类型：TEXT | 格式：RFC3339 | 用于检查会话是否已过期
        // - created_at: 会话创建时间 | 类型：TEXT | 格式：RFC3339 | 用于审计和日志
        // - FOREIGN KEY: user_id 必须 exist in users.id，且当用户被删除时 CASCADE 级联删除相关会话
        `CREATE TABLE IF NOT EXISTS sessions (
            id TEXT PRIMARY KEY,
            user_id INTEGER NOT NULL,
            expires_at TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        );`,

        // === certificates 表 === | 用途：存储 SSL/TLS 证书的元数据和信息
        // 表结构说明：
        // - id: 证书唯一标识 | 类型：INTEGER | 自增主键
        // - primary_domain: 主域名 | 类型：TEXT | 值：example.com | 用于证书分组和目录组织
        // - domains: 所有包含的域名 | 类型：TEXT | 值：JSON 数组字符串 ["example.com", "www.example.com"] | 用于申请多域名/通配符证书
        // - email: ACME 账户邮箱 | 类型：TEXT | 值：admin@example.com | Let's Encrypt 用于发送过期通知
        // - cert_dir: 本地存储目录 | 类型：TEXT | 值：./data/certs/example.com | 用于定位证书文件
        // - fullchain_path: 完整证书链路径 | 类型：TEXT | 值：./data/certs/example.com/fullchain.pem | 包含服务器证书 + 中间证书
        // - privkey_path: 私钥文件路径 | 类型：TEXT | 值：./data/certs/example.com/privkey.pem | 敏感数据，仅内部使用
        // - expires_at: 证书过期时间 | 类型：TEXT | 格式：RFC3339 | 可为 NULL（新证书申请中）| 用于续期判断
        // - status: 证书状态 | 类型：TEXT | 值：issued（已颁发）| renewed（已续期）| error（申请失败）
        // - last_error: 最后一次申请/续期的错误信息 | 类型：TEXT | 可为 NULL | 用户查看失败原因
        // - last_renewed_at: 最后一次成功续期时间 | 类型：TEXT | 格式：RFC3339 | 可为 NULL（首次申请则为空）| tracking 续期历史
        // - created_at: 证书创建时间 | 类型：TEXT | 格式：RFC3339 | = 首次申请时间
        // - updated_at: 最后一次更新时间 | 类型：TEXT | 格式：RFC3339 | 任何修改（包括续期）都更新此时间戳
        `CREATE TABLE IF NOT EXISTS certificates (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            primary_domain TEXT NOT NULL,
            domains TEXT NOT NULL,
            email TEXT NOT NULL,
            cert_dir TEXT NOT NULL,
            fullchain_path TEXT NOT NULL,
            privkey_path TEXT NOT NULL,
            expires_at TEXT,
            status TEXT NOT NULL,
            last_error TEXT,
            last_renewed_at TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );`,
    }

    // 遍历所有 SQL 语句并执行
    // 每个 CREATE TABLE IF NOT EXISTS 语句会：
    // 1. 如果表已存在，则不做任何操作
    // 2. 如果表不存在，则创建新表
    for _, query := range queries {
        // 执行单个 DDL（数据定义语言）SQL 语句
        // db.Exec 返回 sql.Result （包含受影响行数等信息）和错误
        // CREATE TABLE 语句不返回行数，这里忽略第一个返回值（_）
        if _, err := db.Exec(query); err != nil {
            // 如果 SQL 执行失败，返回详细的错误信息
            // 常见原因：SQL 语法错误、磁盘空间不足、I/O 错误
            return fmt.Errorf("执行迁移失败: %w", err)
        }
    }

    // 所有表都成功创建（或已存在），迁移完成
    return nil
}
