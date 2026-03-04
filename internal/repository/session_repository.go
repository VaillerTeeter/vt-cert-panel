// 包：repository | 用途：数据访问层（DAO）- 负责所有数据库操作 | 关键接口/结构：CertificateRepository、UserRepository、SessionRepository
// 关键职责：1）将数据模型持久化到数据库（CRUD 操作）| 2）提供数据查询接口 | 3）处理数据库类型与 Go 类型的转换
package repository

// === 依赖包导入 === | 说明：会话数据持久化所需的核心包
import (
    // database/sql 包提供通用的数据库操作接口 | 用途：执行 SQL 查询、删除、修改等数据库操作
    // | 来自：Go 标准库
    "database/sql"
    // time 包处理时间操作 | 用途：时间戳转换（time.RFC3339 格式）、时间解析和比较、过期检查
    // | 来自：Go 标准库
    "time"
)

// SessionRepository 是会话数据访问对象（DAO）
// 用途：管理会话数据的数据库读写操作，实现会话的创建、查询、删除、过期清理等操作
// 持有的资源：db = 数据库连接池
type SessionRepository struct {
    // db 表示数据库连接池对象 | 类型：*sql.DB | 作用域：所有 SQL 操作
    // 由应用启动时创建，所有数据库操作都通过此连接池进行
    db *sql.DB
}

// NewSessionRepository 是 SessionRepository 的构造函数
// 输入：db = 数据库连接池
// 输出：返回初始化的 *SessionRepository 实例
// 说明：简单的依赖注入，将数据库连接池关联到仓库中供后续操作使用
func NewSessionRepository(db *sql.DB) *SessionRepository {
    // 创建并返回 SessionRepository 实例，关联数据库连接
    return &SessionRepository{db: db}
}

// (r *SessionRepository) Create 用于创建新的会话记录
// 输入：id = 会话 ID（唯一标识符）、userID = 用户 ID（关联的用户）、expiresAt = 会话过期时间
// 输出：返回错误或 nil（创建成功）
// 说明：执行 INSERT 操作，将会话信息保存到数据库
func (r *SessionRepository) Create(id string, userID int64, expiresAt time.Time) error {
    // 执行 INSERT SQL 语句，将会话数据插入 sessions 表
    // 使用参数化查询（?占位符）防止 SQL 注入
    _, err := r.db.Exec(`INSERT INTO sessions(id, user_id, expires_at, created_at) VALUES(?,?,?,?)`, id, userID, expiresAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
    // 直接返回执行结果的错误值（如果有的话）
    return err
}

// (r *SessionRepository) GetUserID 用于查询会话关联的用户 ID 和过期时间
// 输入：sessionID = 会话的唯一标识符
// 输出：返回用户 ID、会话过期时间和可能的错误（如果会话不存在则返回 sql.ErrNoRows）
// 说明：用于检查会话有效性和验证用户身份
func (r *SessionRepository) GetUserID(sessionID string) (int64, time.Time, error) {
    // 执行 SELECT SQL 查询，根据会话 ID 查询用户 ID 和过期时间
    row := r.db.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE id=?`, sessionID)

    // 定义临时变量存储查询结果
    // userID：用户的唯一标识符
    var userID int64
    // expires：从数据库读出的过期时间字符串（RFC3339 格式）
    var expires string

    // 从数据库行扫描数据到对应变量
    if err := row.Scan(&userID, &expires); err != nil {
        // 如果扫描失败（会话不存在或数据损坏），返回错误
        return 0, time.Time{}, err
    }

    // 解析过期时间字符串 为 time.Time 对象
    // 用于后续的过期检查
    expiresAt, _ := time.Parse(time.RFC3339, expires)

    // 返回用户 ID、过期时间和 nil 错误
    return userID, expiresAt, nil
}

// (r *SessionRepository) Delete 用于删除指定的会话记录
// 输入：sessionID = 要删除的会话的唯一标识符
// 输出：返回错误或 nil（删除成功）
// 说明：执行 DELETE 操作，在用户登出或会话失效时调用
func (r *SessionRepository) Delete(sessionID string) error {
    // 执行 DELETE SQL 语句，删除指定 ID 的会话
    // 使用参数化查询防止 SQL 注入
    _, err := r.db.Exec(`DELETE FROM sessions WHERE id=?`, sessionID)
    // 直接返回执行结果的错误值（如果有的话）
    return err
}

// (r *SessionRepository) CleanupExpired 用于删除所有已过期的会话
// 输入：无
// 输出：返回错误或 nil（清理成功）
// 说明：定期调用此方法清理过期的会话记录，释放数据库空间，通常由后台定时任务调用
func (r *SessionRepository) CleanupExpired() error {
    // 执行 DELETE SQL 语句，删除所有过期时间早于当前时间的会话
    // 比较：expires_at < 当前时间
    // 这保证了已过期的会话都会被删除
    _, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
    // 直接返回执行结果的错误值（如果有的话）
    return err
}
