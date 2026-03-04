// 包：auth | 用途：用户认证相关功能（密码哈希与验证） | 关键函数：HashPassword、VerifyPassword
package auth

// === 依赖包导入 === | 说明：密码管理所需的密码学包
import (
    // bcrypt 包提供 bcrypt 密码哈希功能 | 用途：生成密码哈希值（GenerateFromPassword）、验证密码（CompareHashAndPassword）
    // | 特点：自适应算法，可抵御暴力攻击和时序攻击 | 来自：golang.org/x/crypto 外部包
    "golang.org/x/crypto/bcrypt"
)

// HashPassword 用于将明文密码哈希为 bcrypt 哈希值
// 输入：password = 用户明文密码
// 输出：返回哈希值字符串，失败时返回错误
// 可能失败：bcrypt 库故障（极少发生）
func HashPassword(password string) (string, error) {
    // 使用 bcrypt 默认成本参数（DefaultCost = 10）生成哈希值
    // 更高的成本值 = 更强的安全性但计算耗时更长
    hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        // 如果哈希生成失败，返回错误
        return "", err
    }
    // 返回哈希值，可直接存储到数据库
    return string(hashed), nil
}

// VerifyPassword 用于验证明文密码是否与已存储的哈希值匹配
// 输入：hash = 数据库中存储的 bcrypt 哈希值，password = 用户输入的明文密码
// 输出：返回 true 表示密码正确，false 表示密码错误
// 可能失败：无（总是返回 bool，不返回错误）
func VerifyPassword(hash string, password string) bool {
    // 使用 bcrypt.CompareHashAndPassword 进行常时间比较（防止时序攻击）
    // 返回 nil = 密码正确，返回 error = 密码错误
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
