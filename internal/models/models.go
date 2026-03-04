// 包：models | 用途：数据模型定义（User、Certificate） | 关键对象：User、Certificate
package models

import "time"

// User 表示系统中的单个用户
// 关键字段：ID = 数据库主键，Username = 用户名，PasswordHash = bcrypt 哈希密码，CreatedAt = 创建时间
type User struct {
    // ID 表示用户的唯一标识 | 类型：int64（SQLite rowid） | 作用域：全局唯一
    ID int64
    // Username 表示用户的登录名 | 类型：string | 作用域：全局唯一
    Username string
    // PasswordHash 表示经过 bcrypt 哈希后的密码 | 类型：string | 作用域：内部使用，永不出现在 API 响应
    PasswordHash string
    // CreatedAt 表示用户创建时间 | 类型：time.Time（UTC）| 作用域：used for tracking
    CreatedAt time.Time
}

// Certificate 表示一个 SSL/TLS 证书及其元数据
// 关键字段：ID = 证书 ID，PrimaryDomain = 主域名，Domains = 所有包含的域名，ExpiresAt = 过期时间，Status = 证书状态
type Certificate struct {
    // ID 表示证书的唯一标识 | 类型：int64（SQLite rowid）| 作用域：全局唯一 | 返回给前端：是
    ID int64 `json:"id"`
    // PrimaryDomain 表示证书的主域名（如 example.com）| 类型：string | 作用域：used for directory organization | 返回给前端：是
    PrimaryDomain string `json:"primaryDomain"`
    // Domains 表示证书包含的所有域名列表（包括主域名和 SANs）| 类型：[]string | 返回给前端：是
    Domains []string `json:"domains"`
    // Email 表示 ACME 帐户邮箱（Let's Encrypt 用于发送过期通知）| 类型：string | 返回给前端：是
    Email string `json:"email"`
    // CertDir 表示本地文件系统中证书的存储目录 | 类型：string | 作用域：内部使用 | 返回给前端：否（json:"-"）
    CertDir string `json:"-"`
    // FullchainPath 表示证书链文件的完整路径（fullchain.pem）| 类型：string | 作用域：内部使用 | 返回给前端：否
    FullchainPath string `json:"-"`
    // PrivkeyPath 表示私钥文件的完整路径（privkey.pem）| 类型：string | 作用域：内部使用，绝不在网络上传输 | 返回给前端：否
    PrivkeyPath string `json:"-"`
    // ExpiresAt 表示证书的过期时间（UTC）| 类型：time.Time | 返回给前端：是 | 用于续期计算
    ExpiresAt time.Time `json:"expiresAt"`
    // Status 表示证书的当前状态（issued = 已颁发，renewed = 已续期，error = 申请失败）| 类型：string | 返回给前端：是
    Status string `json:"status"`
    // LastError 表示最后一次申请/续期的错误信息（失败时有值，成功时为空）| 类型：string | 返回给前端：是（omitempty = 空值不包含在 JSON） | 作用域：用户查看失败原因
    LastError string `json:"lastError,omitempty"`
    // LastRenewedAt 表示最后一次成功续期的时间（首次申请时为空）| 类型：time.Time（UTC）| 返回给前端：是（omitempty） | 作用域：tracking 续期历史
    LastRenewedAt time.Time `json:"lastRenewedAt,omitempty"`
    // CreatedAt 表示证书在系统中的创建时间（首次申请时）| 类型：time.Time（UTC）| 返回给前端：是 | 作用域：tracking
    CreatedAt time.Time `json:"createdAt"`
    // UpdatedAt 表示证书最后一次更新的时间（任何修改包括续期都会更新）| 类型：time.Time（UTC）| 返回给前端：是 | 作用域：tracking
    UpdatedAt time.Time `json:"updatedAt"`
}
