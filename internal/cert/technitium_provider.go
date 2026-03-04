// 包：cert | 用途：ACME 证书管理（申请、续期、密钥生成）| 关键接口/结构：TechnitiumProvider、Service
// 关键职责：1）实现 lego 的 DNS-01 完整性验证接口 | 2）管理与 Technitium DNS 的交互 | 3）处理 RSA 密钥与 PEM 编码
package cert

// === 依赖包导入 === | 说明：DNS-01 ACME 验证所需的时间处理和 lego 库包
import (
    // time 包提供时间处理功能 | 用途：时间间隔计算、DNS 传播延迟杜度控制（time.Duration）
    // | 来自：Go 标准库
    "time"
    // lego v4 的 DNS-01 challenge 包 | 用途：实现 lego 库的 DNS-01 完整性验证接口（Challenge 接口）
    // | 关键函数：dns01.GetRecord() - 计算 ACME 验证记录的域名和值 | 来自：github.com/go-acme/lego/v4
    "github.com/go-acme/lego/v4/challenge/dns01"
    // dns 包包含 Technitium DNS 客户端实现 | 用途：创建和使用 TechnitiumClient 实例进行 DNS 记录操作（AddTXTRecord、DeleteTXTRecord）
    // | 来自：内部模块 vt-cert-panel/internal/dns
    "vt-cert-panel/internal/dns"
)

// TechnitiumProvider 实现 lego 库的 DNS-01 完整性验证接口
// 用于 ACME 流程中的 DNS 记录管理（添加/删除临时 TXT 记录）
// 关键字段：client = DNS 客户端、propagationSec = DNS 传播等待时间
type TechnitiumProvider struct {
    // client 表示与 Technitium DNS API 交互的客户端 | 类型：*dns.TechnitiumClient | 作用域：内部使用
    client         *dns.TechnitiumClient
    // propagationSec 表示 DNS 记录添加后需要等待的传播时间（单位：秒）| 类型：int | 作用域：内部使用
    // 用于在 ACME 验证前等待 DNS 全球传播，减少验证失败风险
    propagationSec int
}

// NewTechnitiumProvider 用于创建 Technitium DNS-01 提供者实例
// 输入：client = DNS 客户端实例、propagationSec = DNS 传播等待时间（单位：秒）
// 输出：返回初始化的 *TechnitiumProvider
// 可能失败：无
func NewTechnitiumProvider(client *dns.TechnitiumClient, propagationSec int) *TechnitiumProvider {
    // 创建并返回 TechnitiumProvider 实例
    return &TechnitiumProvider{client: client, propagationSec: propagationSec}
}

// (p *TechnitiumProvider) Present 用于在 ACME DNS-01 验证时添加临时 TXT 记录
// 输入：domain = 域名、token = ACME token（未使用）、keyAuth = ACME 密钥授权字符串
// 输出：返回错误或 nil（成功）
// 可能失败：1）DNS 记录添加失败 | 2）计算 FQDN/记录值失败
func (p *TechnitiumProvider) Present(domain, token, keyAuth string) error {
    // 计算完整的 FQDN（如 _acme-challenge.example.com）和记录值（base64 编码的哈希）
    fqdn, value, err := computeFQDN(domain, keyAuth)
    if err != nil {
        // 如果计算失败，返回错误
        return err
    }

    // 调用 Technitium DNS 客户端添加 TXT 记录
    // 该记录用于 ACME 服务器验证域名所有权
    if err := p.client.AddTXTRecord(fqdn, value); err != nil {
        // 如果添加失败，返回错误
        return err
    }

    // DNS 记录添加成功
    return nil
}

// (p *TechnitiumProvider) CleanUp 用于在 ACME DNS-01 验证完成后删除临时 TXT 记录
// 输入：domain = 域名、token = ACME token（未使用）、keyAuth = ACME 密钥授权字符串
// 输出：返回错误或 nil（成功）
// 可能失败：1）DNS 记录删除失败 | 2）计算 FQDN/记录值失败
func (p *TechnitiumProvider) CleanUp(domain, token, keyAuth string) error {
    // 计算完整的 FQDN（如 _acme-challenge.example.com）和记录值（base64 编码的哈希）
    fqdn, value, err := computeFQDN(domain, keyAuth)
    if err != nil {
        // 如果计算失败，返回错误
        return err
    }
    // 调用 Technitium DNS 客户端删除 TXT 记录
    return p.client.DeleteTXTRecord(fqdn, value)
}

// (p *TechnitiumProvider) Timeout 返回 DNS-01 验证的超时配置
// 输出：timeout = 最大等待时间、interval = 检查间隔
// 这两个参数用于 lego 库的验证流程控制
func (p *TechnitiumProvider) Timeout() (timeout, interval time.Duration) {
    // 最大等待时间 = propagationSec + 额外时间（lego 内部处理）
    // interval = 每 5 秒检查一次 DNS 记录是否已传播
    return time.Duration(p.propagationSec) * time.Second, 5 * time.Second
}

// computeFQDN 是一个辅助函数，用于计算 ACME DNS-01 的完整域名和记录值
// 输入：domain = 原始域名、keyAuth = ACME 密钥授权字符串
// 输出：fqdn = 完整的挑战域名、value = 要存储的记录值、err = 错误
// 使用 lego 库提供的 dns01.GetRecord 进行计算
func computeFQDN(domain, keyAuth string) (string, string, error) {
    // lego 库中的标准函数，计算 _acme-challenge.{domain} 和对应的记录值
    // 返回的记录值是 keyAuth 的 SHA256 哈希的 Base64 编码
    return dns01.GetRecord(domain, keyAuth)
}
