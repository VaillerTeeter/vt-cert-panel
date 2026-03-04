// 包：repository | 用途：数据访问层（DAO）- 负责所有数据库操作 | 关键接口/结构：CertificateRepository、UserRepository、SessionRepository
// 关键职责：1）将数据模型持久化到数据库（CRUD 操作）| 2）提供数据查询接口 | 3）处理数据库类型与 Go 类型的转换
package repository

// === 依赖包导入 === | 说明：用户数据持久化所需的核心包
import (
    // database/sql 包提供通用的数据库操作接口 | 用途：执行 SQL 查询、插入、修改等数据库操作
    // | 来自：Go 标准库
    "database/sql"
    // errors 包提供错误处理功能 | 用途：errors.Is 函数用于检查 SQL 错误类型（如 ErrNoRows）
    // | 来自：Go 标准库
    "errors"
    // time 包处理时间操作 | 用途：时间戳转换（time.RFC3339 格式）、时间解析
    // | 来自：Go 标准库
    "time"
    // models 包（内部）定义数据结构 | 用途：User 模型结构体（对应数据库 users 表）
    // | 来自：内部模块 vt-cert-panel/internal/models
    "vt-cert-panel/internal/models"
)

// UserRepository 是用户数据访问对象（DAO）
// 用途：管理用户数据的数据库读写操作，实现用户的创建、查询、计数等操作
// 持有的资源：db = 数据库连接池
type UserRepository struct {
    // db 表示数据库连接池对象 | 类型：*sql.DB | 作用域：所有 SQL 操作
    // 由应用启动时创建，所有数据库操作都通过此连接池进行
    db *sql.DB
}

// NewUserRepository 是 UserRepository 的构造函数
// 输入：db = 数据库连接池
// 输出：返回初始化的 *UserRepository 实例
// 说明：简单的依赖注入，将数据库连接池关联到仓库中供后续操作使用
func NewUserRepository(db *sql.DB) *UserRepository {
    // 创建并返回 UserRepository 实例，关联数据库连接
    return &UserRepository{db: db}
}

// (r *UserRepository) Count 用于统计数据库中的用户总数
// 输入：无
// 输出：返回用户总数和可能的错误
// 说明：用于初始化检查，判断系统是否已创建过管理员用户
func (r *UserRepository) Count() (int, error) {
    // 定义临时变量存储 COUNT 查询结果
    var count int
    // 执行 COUNT SQL 查询，统计 users 表中的记录数
    // COUNT(1) 是一种高效的计数方式，返回表中的总行数
    err := r.db.QueryRow(`SELECT COUNT(1) FROM users`).Scan(&count)
    // 返回用户总数和可能的错误
    return count, err
}

// (r *UserRepository) Create 用于创建新的用户记录
// 输入：username = 用户名、passwordHash = 密码哈希值（已通过 bcrypt 加密）
// 输出：返回错误或 nil（创建成功）
// 说明：执行 INSERT 操作，将用户信息保存到数据库
func (r *UserRepository) Create(username string, passwordHash string) error {
    // 执行 INSERT SQL 语句，将用户数据插入 users 表
    // 使用参数化查询（?占位符）防止 SQL 注入
    // 字段说明：username = 用户名、password_hash = 密码哈希、created_at = 创建时间戳
    _, err := r.db.Exec(`INSERT INTO users(username, password_hash, created_at) VALUES(?,?,?)`, username, passwordHash, time.Now().UTC().Format(time.RFC3339))
    // 直接返回执行结果的错误值（如果有的话）
    return err
}

// (r *UserRepository) GetByUsername 用于根据用户名查询用户信息
// 输入：username = 用户名（唯一标识）
// 输出：返回用户对象或错误；如果用户不存在返回 nil（不返回错误）
// 说明：用于登录时验证用户身份和获取密码哈希进行密码验证
func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
    // 执行 SELECT SQL 查询，根据用户名查询用户信息
    // QueryRow 用于查询单行记录
    row := r.db.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username=?`, username)

    // 定义临时变量存储查询结果中的时间字段（RFC3339 格式字符串）
    var createdAt string

    // 创建用户对象容器
    user := &models.User{}

    // 从数据库行扫描所有 4 个列的数据到对应的变量中
    // 列顺序：id, username, password_hash, created_at
    if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt); err != nil {
        // 如果扫描失败，检查错误类型
        if errors.Is(err, sql.ErrNoRows) {
            // 如果用户不存在（ErrNoRows），返回 nil 而不是错误
            // 这样调用方可以区分"用户不存在"（nil）和"数据库错误"（err != nil）
            return nil, nil
        }
        // 如果是其他错误（数据库连接错误、扫描类型错误等），返回错误
        return nil, err
    }

    // 解析 createdAt 时间字符串为 time.Time 对象
    parsed, _ := time.Parse(time.RFC3339, createdAt)
    // 将解析后的时间赋值给用户对象
    user.CreatedAt = parsed

    // 返回用户对象和 nil 错误
    return user, nil
}
