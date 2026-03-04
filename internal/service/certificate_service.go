// 文件：internal/service/certificate_service.go | 用途：证书业务编排服务 | 关键对象：CertificateService, parseDomains
// 包：service | 用途：业务逻辑层，衔接 repository 与 cert 子系统 | 关键接口/结构：CertificateService
package service

import (
    // archive/zip 包用于构建 ZIP 压缩包
    // 用途：将证书文件与私钥打包为下载文件
    "archive/zip"
    // bytes 包提供内存字节缓冲区
    // 用途：在内存中构造 ZIP 文件内容
    "bytes"
    // fmt 包提供格式化错误能力
    // 用途：返回用户可读的输入校验错误
    "fmt"
    // os 包提供文件系统读写能力
    // 用途：读取 fullchain.pem 和 privkey.pem 文件
    "os"
    // strings 包提供字符串处理函数
    // 用途：域名拆分、标准化、小写化、文件名安全替换
    "strings"
    // time 包提供时间计算能力
    // 用途：计算续期窗口截止时间
    "time"
    // cert 包提供证书签发/续期核心能力
    // 用途：调用 Issue / Renew 执行 ACME 证书操作
    "vt-cert-panel/internal/cert"
    // models 包定义核心数据模型
    // 用途：返回 Certificate 业务对象
    "vt-cert-panel/internal/models"
    // repository 包提供数据库访问能力
    // 用途：持久化证书、查询待续期证书、按 ID 读取证书
    "vt-cert-panel/internal/repository"
)

// CertificateService 表示证书业务服务 | 关键字段：repo（证书仓库）、issuer（证书签发器）
type CertificateService struct {
    // repo 表示证书仓库 | 类型：*repository.CertificateRepository | 作用域：数据库读写
    repo    *repository.CertificateRepository
    // issuer 表示证书签发服务 | 类型：*cert.Service | 作用域：证书申请与续期
    issuer  *cert.Service
}

// NewCertificateService 用于创建证书业务服务 | 输入：repo 证书仓库、issuer 签发服务 | 输出：*CertificateService | 可能失败：无
func NewCertificateService(repo *repository.CertificateRepository, issuer *cert.Service) *CertificateService {
    // 创建并返回服务实例，完成依赖注入
    return &CertificateService{repo: repo, issuer: issuer}
}

// Create 用于创建并签发新证书 | 输入：domainsInput 域名输入字符串、email 申请邮箱 | 输出：已创建证书对象 | 可能失败：域名为空、签发失败、数据库写入失败
func (s *CertificateService) Create(domainsInput string, email string) (*models.Certificate, error) {
    // 解析用户输入的域名字符串（支持逗号、空格、换行等分隔）
    domains := parseDomains(domainsInput)
    if len(domains) == 0 {
        // 如果解析后没有有效域名，则返回输入错误
        return nil, fmt.Errorf("请输入至少一个域名")
    }

    // 调用签发器申请证书（包含 DNS-01 验证与证书文件落盘）
    result, err := s.issuer.Issue(domains, email)
    if err != nil {
        // 如果签发失败，则透传错误给上层
        return nil, err
    }

    // 将证书元数据写入数据库，获取自增 ID
    id, err := s.repo.Create(result)
    if err != nil {
        // 如果数据库写入失败，则返回错误
        return nil, err
    }

    // 回填数据库主键到返回对象，保证调用方拿到完整数据
    result.ID = id

    // 返回创建成功的证书对象
    return result, nil
}

// List 用于查询证书列表 | 输入：无 | 输出：证书数组 | 可能失败：数据库查询失败
func (s *CertificateService) List() ([]models.Certificate, error) {
    // 直接委托给仓库层执行查询
    return s.repo.List()
}

// RenewDue 用于续期即将到期证书 | 输入：beforeDays 续期窗口天数（相对今天） | 输出：error | 可能失败：查询失败
func (s *CertificateService) RenewDue(beforeDays int) error {
    // 计算续期截止时间点：当前时间 + beforeDays
    // 例如 beforeDays=30 表示“30 天内到期”的证书都要续期
    due, err := s.repo.FindDueForRenew(time.Now().AddDate(0, 0, beforeDays))
    if err != nil {
        // 如果待续期列表查询失败，直接返回
        return err
    }

    // 遍历所有待续期证书，逐个尝试续期
    for i := range due {
        // 调用签发器续期当前证书
        updated, renewErr := s.issuer.Renew(&due[i])
        if renewErr != nil {
            // 如果续期失败，记录错误状态，便于前端展示和后续排障
            due[i].Status = "error"
            // 将具体失败信息写入 last_error
            due[i].LastError = renewErr.Error()
            // 尝试将失败状态回写数据库（此处忽略更新错误，避免影响后续条目处理）
            _ = s.repo.Update(&due[i])
            // 继续处理下一张证书，不中断整体续期流程
            continue
        }

        // 续期成功后，回写最新证书信息（状态、路径、过期时间等）
        _ = s.repo.Update(updated)
    }

    // 全部处理完成
    return nil
}

// BuildZip 用于构建证书下载压缩包 | 输入：id 证书 ID | 输出：zip 二进制内容、文件名 | 可能失败：记录不存在、文件读取失败、zip 构建失败
func (s *CertificateService) BuildZip(id int64) ([]byte, string, error) {
    // 按 ID 读取证书记录，拿到证书与私钥文件路径
    item, err := s.repo.GetByID(id)
    if err != nil {
        // 如果记录查询失败（例如不存在），返回错误
        return nil, "", err
    }

    // 读取证书链文件内容
    fullchain, err := os.ReadFile(item.FullchainPath)
    if err != nil {
        // 如果证书链文件读取失败，返回错误
        return nil, "", err
    }

    // 读取私钥文件内容
    privkey, err := os.ReadFile(item.PrivkeyPath)
    if err != nil {
        // 如果私钥文件读取失败，返回错误
        return nil, "", err
    }

    // 创建内存缓冲区作为 ZIP 输出目标
    var buf bytes.Buffer
    // 创建 ZIP 写入器
    zw := zip.NewWriter(&buf)
    // 定义需要打包的文件集合（压缩包内文件名 -> 文件内容）
    files := map[string][]byte{"fullchain.pem": fullchain, "privkey.pem": privkey}

    // 遍历文件集合并写入 ZIP
    for name, content := range files {
        // 在 ZIP 中创建一个文件条目
        entry, err := zw.Create(name)
        if err != nil {
            // 如果 ZIP 条目创建失败，返回错误
            return nil, "", err
        }

        // 将文件内容写入 ZIP 条目
        if _, err := entry.Write(content); err != nil {
            // 如果写入失败，返回错误
            return nil, "", err
        }
    }

    // 关闭 ZIP 写入器，刷新尾部结构
    if err := zw.Close(); err != nil {
        // 如果关闭失败，返回错误
        return nil, "", err
    }

    // 生成下载文件名：将通配符 * 替换为 wildcard，避免文件名特殊字符问题
    filename := strings.ReplaceAll(item.PrimaryDomain, "*", "wildcard") + ".zip"

    // 返回压缩包字节流与建议文件名
    return buf.Bytes(), filename, nil
}

// parseDomains 用于解析并规范化域名输入 | 输入：用户输入字符串 | 输出：去重后的域名切片 | 可能失败：无（非法片段会被过滤）
func parseDomains(input string) []string {
    // 使用多分隔符拆分输入：逗号、换行、回车、制表符、空格
    parts := strings.FieldsFunc(input, func(r rune) bool {
        return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
    })

    // set 表示去重集合 | 类型：map[string]struct{} | 作用域：当前函数
    set := map[string]struct{}{}
    // result 表示最终域名结果 | 类型：[]string | 作用域：当前函数
    result := make([]string, 0, len(parts))

    // 遍历每个拆分片段，执行标准化与去重
    for _, part := range parts {
        // 统一转小写并去除首尾空白，保证域名比较一致
        domain := strings.ToLower(strings.TrimSpace(part))
        if domain == "" {
            // 空片段直接跳过
            continue
        }

        if _, exists := set[domain]; exists {
            // 已存在的域名跳过，避免重复申请 SAN
            continue
        }

        // 记录到去重集合
        set[domain] = struct{}{}
        // 追加到结果列表（保持首次出现顺序）
        result = append(result, domain)
    }

    // 返回去重且标准化后的域名数组
    return result
}
