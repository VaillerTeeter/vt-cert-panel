// 包：dns | 用途：与 Technitium DNS 服务交互（TXT 记录的添加/删除）| 关键函数：AddTXTRecord、DeleteTXTRecord
// 关键职责：1）封装 Technitium DNS API 调用 | 2）处理 HTTP 请求/响应 | 3）错误处理与区域查找
package dns

// === 依赖包导入 === | 说明：DNS API 客户端所需的网络通信和数据处理包
import (
    // encoding/json 包处理 JSON 编码和解码 | 用途：解析 Technitium API 的 JSON 响应（json.NewDecoder）
    // | 来自：Go 标准库
    "encoding/json"
    // fmt 包提供格式化输出功能 | 用途：错误消息格式化（fmt.Errorf）、格式化 TTL 值（fmt.Sprintf）
    // | 来自：Go 标准库
    "fmt"
    // net/http 包提供 HTTP 客户端功能 | 用途：创建 HTTP POST 请求（http.NewRequest）、执行请求（http.Client.Do）
    // | 来自：Go 标准库
    "net/http"
    // net/url 包处理 URL 和查询参数 | 用途：构建 URL 编码的表单参数（url.Values）、参数编码（params.Encode）
    // | 来自：Go 标准库
    "net/url"
    // strings 包处理字符串操作 | 用途：移除末尾字符（strings.TrimSuffix）、分割域名（strings.Split）、去空格（strings.TrimSpace）
    // | 来自：Go 标准库
    "strings"
    // time 包处理时间操作 | 用途：设置 HTTP 请求超时（time.Second）
    // | 来自：Go 标准库
    "time"
    // config 包包含应用程序配置类型 | 用途：获取 TechnitiumConfig 结构体，包含 BaseURL、Token、DefaultTTL 配置
    // | 来自：内部模块 vt-cert-panel/internal/config
    "vt-cert-panel/internal/config"
)

// TechnitiumClient 表示与 Technitium DNS API 交互的 HTTP 客户端
// 用于 ACME DNS-01 验证过程中添加和删除临时 TXT 记录
// 关键字段：baseURL = API 服务地址、token = API 认证 token、ttl = 默认 TTL、client = HTTP 客户端
type TechnitiumClient struct {
    // baseURL 表示 Technitium DNS 服务的基础 URL（如 http://x.x.x.x:5380）| 类型：string | 作用域：内部使用
    // 不包含末尾 / 符号
    baseURL string
    // token 表示 Technitium API 的认证 token（API Key）| 类型：string | 作用域：内部使用
    // ⚠️ 敏感信息，不能泄露
    token   string
    // ttl 表示添加的 DNS TXT 记录的默认生存时间（单位：秒）| 类型：int | 作用域：内部使用
    ttl     int
    // client 表示 HTTP 客户端，用于发送 API 请求 | 类型：*http.Client | 作用域：内部使用
    // 包含 20 秒超时设置，防止请求无限阻塞
    client  *http.Client
}

// apiResponse 表示 Technitium API 的响应结构
// 包含操作状态和错误信息
type apiResponse struct {
    // Status 表示操作状态 | 类型：string | 值：\"ok\" 表示成功，其他值表示失败
    Status       string `json:"status"`
    // ErrorMessage 表示错误描述信息（仅在失败时有值）| 类型：string | 作用域：错误诊断
    ErrorMessage string `json:"errorMessage"`
}

// NewTechnitiumClient 用于创建 Technitium DNS 客户端实例
// 输入：cfg = Technitium 配置对象（包含 baseUrl、token、defaultTtl）
// 输出：返回初始化的 *TechnitiumClient
// 可能失败：无
func NewTechnitiumClient(cfg config.TechnitiumConfig) *TechnitiumClient {
    // 去掉 baseUrl 末尾的 / 符号，确保 URL 格式一致
    base := strings.TrimRight(cfg.BaseURL, "/")
    // 创建并返回客户端实例，HTTP 客户端超时设置为 20 秒
    return &TechnitiumClient{
        baseURL: base,
        token:   cfg.Token,
        ttl:     cfg.DefaultTTL,
        client:  &http.Client{Timeout: 20 * time.Second},
    }
}

// (c *TechnitiumClient) AddTXTRecord 用于在 Technitium DNS 中添加 TXT 记录
// 输入：fqdn = 完整域名（如 _acme-challenge.example.com）、value = 记录值
// 输出：返回错误或 nil（成功）
// 可能失败：1）HTTP 请求失败 | 2）DNS API 返回错误状态 | 3）网络超时
// 用途：ACME DNS-01 验证需要添加临时 TXT 记录
func (c *TechnitiumClient) AddTXTRecord(fqdn string, value string) error {
    // 查找 FQDN 对应的 DNS 主区域（如 fqdn=_acme-challenge.example.com 时，zone=example.com）
    zone := findZone(fqdn)
    // API 端点：添加 DNS 记录
    endpoint := c.baseURL + "/api/zones/records/add"
    // 构建 POST 请求参数
    params := url.Values{}
    // 设置 API 认证 token（用于身份验证）
    params.Set("token", c.token)
    // 设置完整域名（去掉末尾的 .）
    params.Set("domain", strings.TrimSuffix(fqdn, "."))
    // 设置 DNS 主区域（Zone 的概念）
    params.Set("zone", zone)
    // 设置记录类型为 TXT（Text 记录，ACME 验证专用）
    params.Set("type", "TXT")
    // 设置记录生存时间（TTL），值为秒，越小越快更新但缓存命中率低
    params.Set("ttl", fmt.Sprintf("%d", c.ttl))
    // 设置记录内容（ACME 密钥授权值）
    params.Set("text", value)
    // 设置覆盖标志为 false，表示不覆盖已存在的同名记录（多条记录）
    params.Set("overwrite", "false")
    // 发送 HTTP 请求到 API 端点并获取结果
    return c.call(endpoint, params)
}

// (c *TechnitiumClient) DeleteTXTRecord 用于从 Technitium DNS 中删除 TXT 记录
// 输入：fqdn = 完整域名（如 _acme-challenge.example.com）、value = 记录值
// 输出：返回错误或 nil（成功）
// 可能失败：1）HTTP 请求失败 | 2）DNS API 返回错误状态 | 3）记录不存在（通常不影响后续操作）
// 用途：ACME DNS-01 验证完成后删除临时 TXT 记录
func (c *TechnitiumClient) DeleteTXTRecord(fqdn string, value string) error {
    // 查找 FQDN 对应的 DNS 主区域（与 AddTXTRecord 相同）
    zone := findZone(fqdn)
    // API 端点：删除 DNS 记录
    endpoint := c.baseURL + "/api/zones/records/delete"
    // 构建 POST 请求参数
    params := url.Values{}
    // 设置 API 认证 token（用于身份验证）
    params.Set("token", c.token)
    // 设置完整域名（去掉末尾的 .）
    params.Set("domain", strings.TrimSuffix(fqdn, "."))
    // 设置 DNS 主区域（Zone）
    params.Set("zone", zone)
    // 设置记录类型为 TXT（表示删除 TXT 记录）
    params.Set("type", "TXT")
    // 设置记录内容（用于精确匹配要删除的记录）
    params.Set("text", value)
    // 发送 HTTP 请求到 API 端点并获取结果
    return c.call(endpoint, params)
}

// (c *TechnitiumClient) call 是一个内部辅助函数，用于发送 HTTP POST 请求到 Technitium API
// 输入：endpoint = API 端点 URL、params = POST 参数
// 输出：返回错误或 nil（成功）
// 可能失败：1）HTTP 请求创建失败 | 2）HTTP 请求执行失败 | 3）服务器返回 4xx/5xx 错误 | 4）JSON 解析失败 | 5）API 返回非 ok 状态
func (c *TechnitiumClient) call(endpoint string, params url.Values) error {
    // 创建 HTTP POST 请求
    // params.Encode() 将参数编码为 application/x-www-form-urlencoded 格式
    req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(params.Encode()))
    if err != nil {
        // 如果请求创建失败，返回错误
        return err
    }

    // 设置 Content-Type 头
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    // 执行 HTTP 请求（包含 20 秒超时）
    resp, err := c.client.Do(req)
    if err != nil {
        // 如果 HTTP 请求失败（如网络错误、超时），返回错误
        return err
    }

    // 关闭响应体，防止资源泄露
    defer resp.Body.Close()

    // 检查 HTTP 状态码
    if resp.StatusCode >= 400 {
        // 如果状态码 >= 400，表示服务器错误，返回错误
        return fmt.Errorf("Technitium HTTP错误: %d", resp.StatusCode)
    }

    // 解析 JSON 响应
    var result apiResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        // 如果 JSON 解析失败，返回错误
        return err
    }

    // 检查 API 操作状态
    if result.Status != "ok" {
        // 如果状态不是 \"ok\"，表示操作失败
        // 如果没有错误信息，则使用状态值作为错误信息
        if result.ErrorMessage == "" {
            result.ErrorMessage = result.Status
        }
        // 返回 Technitium 返回的错误信息
        return fmt.Errorf("Technitium接口失败: %s", result.ErrorMessage)
    }

    // 操作成功
    return nil
}

// findZone 是一个辅助函数，用于从完整域名中查找主区域
// 输入：fqdn = 完整域名（如 _acme-challenge.example.com）
// 输出：返回主区域（如 example.com）
// 算法：取最后两部分（如 example.com）或整个域名（如果只有一部分）
func findZone(fqdn string) string {
    // 去掉末尾的 . 和空白字符
    host := strings.TrimSuffix(strings.TrimSpace(fqdn), ".")
    // 按 . 分割域名
    parts := strings.Split(host, ".")

    // 如果只有一部分（不包含 .），直接返回
    if len(parts) < 2 {
        return host
    }

    // 返回最后两部分（如 example.com）
    // 这对大多数情况都适用，对于特殊情况（如 co.uk），可能需要额外处理
    return strings.Join(parts[len(parts)-2:], ".")
}
