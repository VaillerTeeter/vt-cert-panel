// 包：service | 用途：业务逻辑层，编排 repository 与底层能力模块
// 关键职责：1）认证流程控制 2）管理员初始化 3）会话生命周期管理
package service

import (
    // crypto/rand 包用于生成密码学安全随机数
    // 用途：生成高强度随机 sessionID
    "crypto/rand"
    // database/sql 包用于引用标准数据库错误类型
    // 用途：识别 sql.ErrNoRows（记录不存在）
    "database/sql"
    // encoding/hex 包用于二进制到十六进制编码
    // 用途：将随机字节编码为可传输/可存储的 sessionID 字符串
    "encoding/hex"
    // errors 包用于错误创建和比较
    // 用途：构造业务错误、判断错误类型
    "errors"
    // time 包用于会话过期时间计算
    // 用途：计算 TTL、比较是否过期
    "time"
    // auth 包提供密码哈希与校验能力
    // 用途：HashPassword / VerifyPassword
    "vt-cert-panel/internal/auth"
    // config 包提供全局配置
    // 用途：读取会话 TTL 配置
    "vt-cert-panel/internal/config"
    // repository 包提供数据访问能力
    // 用途：用户与会话数据读写
    "vt-cert-panel/internal/repository"
)

// AuthService 是认证业务服务
// 职责：管理员初始化、登录、会话校验、登出
type AuthService struct {
    // cfg 保存全局配置（如会话有效期）
    cfg      *config.Config
    // users 提供用户数据访问能力
    users    *repository.UserRepository
    // sessions 提供会话数据访问能力
    sessions *repository.SessionRepository
}

// NewAuthService 创建 AuthService 实例
// 输入：cfg 配置对象，users 用户仓库，sessions 会话仓库
// 输出：初始化完成的认证服务
func NewAuthService(cfg *config.Config, users *repository.UserRepository, sessions *repository.SessionRepository) *AuthService {
    // 依赖注入：将配置与仓库绑定到服务实例
    return &AuthService{cfg: cfg, users: users, sessions: sessions}
}

// IsInitialized 判断系统是否已经完成管理员初始化
// 规则：users 表中存在任意用户即视为已初始化
func (s *AuthService) IsInitialized() (bool, error) {
    // 查询用户总数
    count, err := s.users.Count()
    if err != nil {
        // 数据库异常时向上返回
        return false, err
    }
    // 用户数大于 0 表示已初始化
    return count > 0, nil
}

// InitializeAdmin 初始化首个管理员账户
// 约束：仅允许在未初始化状态下执行一次
func (s *AuthService) InitializeAdmin(username, password string) error {
    // 先检查是否已初始化
    initialized, err := s.IsInitialized()
    if err != nil {
        return err
    }

    if initialized {
        // 已存在管理员时拒绝重复初始化
        return errors.New("管理员已初始化")
    }

    // 对明文密码做哈希，不保存明文
    hash, err := auth.HashPassword(password)
    if err != nil {
        return err
    }

    // 创建管理员用户记录
    return s.users.Create(username, hash)
}

// Login 执行登录流程并创建会话
// 成功返回：sessionID
// 失败返回：业务错误或底层错误
func (s *AuthService) Login(username, password string) (string, error) {
    // 根据用户名查询用户
    user, err := s.users.GetByUsername(username)
    if err != nil {
        return "", err
    }

    // 用户不存在或密码不匹配均返回统一错误，避免泄露账户存在性
    if user == nil || !auth.VerifyPassword(user.PasswordHash, password) {
        return "", errors.New("用户名或密码错误")
    }

    // 生成 32 字节随机会话令牌（256 bit）
    buf := make([]byte, 32)
    if _, err := rand.Read(buf); err != nil {
        return "", err
    }

    // 编码为十六进制字符串，便于 cookie/数据库传输与存储
    sessionID := hex.EncodeToString(buf)
    // 创建会话，过期时间 = 当前时间 + 配置 TTL
    if err := s.sessions.Create(sessionID, user.ID, time.Now().Add(s.cfg.SessionTTL())); err != nil {
        return "", err
    }

    // 返回会话 ID 给上层（通常用于写入 Cookie）
    return sessionID, nil
}

// ValidateSession 校验会话是否有效
// 返回值：有效性布尔值 + 错误
func (s *AuthService) ValidateSession(sessionID string) (bool, error) {
    // 空 sessionID 直接判定无效
    if sessionID == "" {
        return false, nil
    }

    // 查询会话对应用户与过期时间
    _, expiresAt, err := s.sessions.GetUserID(sessionID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // 会话不存在：视为无效，不当作系统错误
            return false, nil
        }
        // 其他数据库错误向上抛出
        return false, err
    }

    // 过期会话判定无效，并顺手清理
    if expiresAt.Before(time.Now()) {
        _ = s.sessions.Delete(sessionID)
        return false, nil
    }

    // 命中有效会话
    return true, nil
}

// Logout 登出当前会话
// 行为：删除会话记录；空 sessionID 视为无操作
func (s *AuthService) Logout(sessionID string) error {
    if sessionID == "" {
        // 空会话无需处理
        return nil
    }
    // 删除会话记录
    return s.sessions.Delete(sessionID)
}
