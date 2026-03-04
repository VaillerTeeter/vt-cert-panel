// 包：httpx | 用途：HTTP 服务器与路由管理 | 关键接口/结构：Server
// 关键职责：1）处理所有 HTTP 路由和请求 | 2）实现身份验证和授权逻辑 | 3）管理会话生命周期 | 4）提供 REST API 接口
package httpx

// === 依赖包导入 === | 说明：HTTP 服务器和 API 处理所需的核心包
import (
    // context 包用于管理请求和超时 | 用途：传递请求上下文、优雅关闭超时（Shutdown）
    // | 来自：Go 标准库
    "context"
    // encoding/json 包处理 JSON 编码和解码 | 用途：解析 JSON 请求体、编码 JSON 响应
    // | 来自：Go 标准库
    "encoding/json"
    // fmt 包提供格式化输出功能 | 用途：错误消息格式化、字符串拼接
    // | 来自：Go 标准库
    "fmt"
    // html/template 包处理 HTML 模板 | 用途：解析 web/index.html 模板、渲染 HTML 页面
    // | 来自：Go 标准库
    "html/template"
    // net/http 包提供 HTTP 服务器功能 | 用途：处理 HTTP 请求/响应、设置 Cookie、管理 HTTP 服务器
    // | 来自：Go 标准库
    "net/http"
    // strconv 包处理字符串和数值转换 | 用途：将字符串转换为整数（ParseInt - 证书 ID 解析）
    // | 来自：Go 标准库
    "strconv"
    // strings 包处理字符串操作 | 用途：修剪字符串（TrimPrefix、TrimSuffix）、分割字符串
    // | 来自：Go 标准库
    "strings"
    // time 包处理时间操作 | 用途：设置 HTTP 超时参数（time.Second）、Cookie 过期时间
    // | 来自：Go 标准库
    "time"
    // service 包包含业务逻辑层服务 | 用途：使用 AuthService（认证）和 CertificateService（证书管理）
    // | 来自：内部模块 vt-cert-panel/internal/service
    "vt-cert-panel/internal/service"
)

// Server 表示 HTTP 服务器实例，处理所有 Web 请求和 API 调用
// 关键字段：auth = 认证服务、certs = 证书服务、tpl = HTML 模板、srv = HTTP 服务器
type Server struct {
    // auth 表示认证服务实例 | 类型：*service.AuthService | 作用域：内部使用
    // 负责用户登录、注册、会话验证等认证相关操作
    auth  *service.AuthService
    // certs 表示证书管理服务实例 | 类型：*service.CertificateService | 作用域：内部使用
    // 负责证书申请、续期、列表查询等证书管理操作
    certs *service.CertificateService
    // tpl 表示 HTML 模板对象 | 类型：*template.Template | 作用域：内部使用
    // 预加载的 web/index.html 模板，用于渲染单页应用（SPA）主页面
    tpl   *template.Template
    // srv 表示 Go 标准库的 HTTP 服务器实例 | 类型：*http.Server | 作用域：内部使用
    // 实际监听 HTTP 请求的服务器，负责接收请求和发送响应
    srv   *http.Server
}

// NewServer 用于创建并初始化 HTTP 服务器实例
// 输入：addr = 监听地址（如 :8080）、auth = 认证服务、certs = 证书管理服务
// 输出：返回初始化的 *Server 实例或错误
// 可能失败：1）HTML 模板文件 web/index.html 不存在或不可读 | 2）模板语法错误
func NewServer(addr string, auth *service.AuthService, certs *service.CertificateService) (*Server, error) {
    // 加载并解析 HTML 模板文件
    // web/index.html 是单页应用（SPA）的主页面，需要预加载以提高性能
    tpl, err := template.ParseFiles("web/index.html")
    if err != nil {
        // 如果模板加载失败（文件不存在或语法错误），返回错误
        return nil, err
    }

    // 创建 Server 实例，将服务和模板关联
    s := &Server{auth: auth, certs: certs, tpl: tpl}
    // 创建 HTTP 多路复用器（Router），用于将 URL 路径映射到对应的处理函数
    mux := http.NewServeMux()
    // 注册路由和处理函数
    // / - 返回 SPA 主页面（HTML）
    mux.HandleFunc("/", s.handleIndex)
    // /api/bootstrap - 检查系统初始化状态（是否有管理员）
    mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
    // /api/bootstrap/admin - 创建初始管理员账户（只允许第一次调用）
    mux.HandleFunc("/api/bootstrap/admin", s.handleBootstrapAdmin)
    // /api/login - 用户登录接口（返回会话 ID）
    mux.HandleFunc("/api/login", s.handleLogin)
    // /api/logout - 用户登出接口（删除会话）
    mux.HandleFunc("/api/logout", s.handleLogout)
    // /api/certificates - 证书列表和创建（需要身份验证）
    mux.HandleFunc("/api/certificates", s.withAuth(s.handleCertificates))
    // /api/certificates/<id>/download - 下载证书 ZIP 包（需要身份验证）
    mux.HandleFunc("/api/certificates/", s.withAuth(s.handleCertificateDownload))
    // /web/ - 静态资源文件服务（CSS、JavaScript、图片等）
    mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))

    // 创建 HTTP 服务器实例
    // Addr = 监听地址和端口（如 :8080）
    // Handler = 路由处理器（mux）
    // ReadHeaderTimeout = 读取请求头的超时时间（10 秒），防止慢客户端或恶意请求无限期占用连接
    s.srv = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

    // 返回初始化完成的 Server 实例
    return s, nil
}

// (s *Server) Run 用于启动 HTTP 服务器并监听请求
// 输入：无（使用 Server 中已初始化的 srv）
// 输出：返回错误或 nil（仅在服务器关闭时返回）
// 说明：这是一个阻塞性调用，会一直等待直到服务器关闭或发生错误
func (s *Server) Run() error {
    // 启动 HTTP 服务器，开始监听指定地址的请求
    // ListenAndServe 是阻塞调用，不会立即返回
    return s.srv.ListenAndServe()
}

// (s *Server) Shutdown 用于优雅地关闭 HTTP 服务器
// 输入：ctx = 关闭超时上下文（如 10 秒超时）
// 输出：返回错误或 nil（关闭成功）
// 说明：会停止接收新请求，但继续处理现有请求直到超时
func (s *Server) Shutdown(ctx context.Context) error {
    // 优雅关闭 HTTP 服务器
    // srv.Shutdown(ctx) 会：
    // 1. 停止接受新连接
    // 2. 等待现有请求完成（最多等待 ctx 的超时时间）
    // 3. 关闭服务器
    return s.srv.Shutdown(ctx)
}

// (s *Server) handleIndex 用于处理 GET / 请求，返回 SPA 主页面
// 输入：w = HTTP 响应写入器、r = HTTP 请求对象
// 输出：写入 HTML 响应到 ResponseWriter
// 说明：返回预加载的 web/index.html 模板，由前端 JavaScript 页面加载 API 数据
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
    // 执行模板，将标题值注入到 HTML 中
    // 模板数据 map 中包含 Title = "VT Cert Panel"
    _ = s.tpl.Execute(w, map[string]string{"Title": "VT Cert Panel"})
}

// (s *Server) handleBootstrap 用于检查系统初始化状态
// 输入：GET 请求（无请求体）
// 输出：返回 {"initialized": true/false} JSON 响应
// 说明：前端在启动时调用此接口检查是否需要创建初始管理员
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
    // 仅允许 GET 请求，其他方法返回 404
    if r.Method != http.MethodGet {
        http.NotFound(w, r)
        return
    }

    // 查询数据库检查是否存在管理员（系统初始化）
    initialized, err := s.auth.IsInitialized()
    if err != nil {
        // 如果查询失败，返回 400 错误
        writeError(w, err)
        return
    }

    // 返回初始化状态
    writeJSON(w, http.StatusOK, map[string]any{"initialized": initialized})
}

// (s *Server) handleBootstrapAdmin 用于创建初始管理员账户
// 输入：POST 请求，JSON 格式：{"username": "admin", "password": "password"}
// 输出：返回 {"ok": true} JSON 响应，或错误信息
// 说明：只有系统未初始化时（不存在管理员）才能调用此接口
func (s *Server) handleBootstrapAdmin(w http.ResponseWriter, r *http.Request) {
    // 仅允许 POST 请求，其他方法返回 404
    if r.Method != http.MethodPost {
        http.NotFound(w, r)
        return
    }

    // 定义请求数据结构
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    // 从请求体解析 JSON
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        // 如果 JSON 格式错误，返回 400 错误
        writeError(w, fmt.Errorf("请求格式错误"))
        return
    }

    // 验证用户名和密码不为空
    if req.Username == "" || req.Password == "" {
        // 如果用户名或密码为空，返回 400 错误
        writeError(w, fmt.Errorf("用户名和密码不能为空"))
        return
    }

    // 创建初始管理员账户
    // 此函数会检查系统是否已初始化，如果已初始化则返回错误
    if err := s.auth.InitializeAdmin(req.Username, req.Password); err != nil {
        // 如果初始化失败（已初始化或数据库错误），返回错误
        writeError(w, err)
        return
    }

    // 初始化成功，返回 OK
    writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// (s *Server) handleLogin 用于用户登录和会话创建
// 输入：POST 请求，JSON 格式：{"username": "admin", "password": "password"}
// 输出：返回 {"ok": true} JSON 响应，或错误信息；同时设置会话 Cookie
// 说明：验证用户名和密码，创建会话并返回 Cookie
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    // 仅允许 POST 请求，其他方法返回 404
    if r.Method != http.MethodPost {
        http.NotFound(w, r)
        return
    }

    // 定义登录请求数据结构
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    // 从请求体解析 JSON
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        // 如果 JSON 格式错误，返回 400 错误
        writeError(w, fmt.Errorf("请求格式错误"))
        return
    }

    // 验证用户名和密码
    // auth.Login() 会检查用户是否存在和密码是否正确，返回会话 ID
    sessionID, err := s.auth.Login(req.Username, req.Password)
    if err != nil {
        // 如果身份验证失败，返回错误（用户不存在或密码错误）
        writeError(w, err)
        return
    }

    // 创建会话 Cookie 并发送给客户端
    // Cookie 包含服务器生成的会话 ID，客户端在后续请求中会自动发回此 Cookie
    http.SetCookie(w, &http.Cookie{
        // Cookie 名称，客户端会使用此名称存储和发送
        Name: "vt_session",
        // Cookie 值为服务器生成的会话 ID
        Value: sessionID,
        // 设置 Path=/，表示整个网站都可以使用此 Cookie
        Path: "/",
        // 设置 HttpOnly=true，禁止 JavaScript 访问（防止 XSS 攻击）⚠️ 安全：防止脚本盗取会话 ID
        HttpOnly: true,
        // 设置 SameSite=Lax，防止 CSRF 攻击（允许同站请求和跨站导航）
        SameSite: http.SameSiteLaxMode,
    })

    // 登录成功，返回 OK 响应
    writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// (s *Server) handleLogout 用于用户登出和会话删除
// 输入：POST 请求（无请求体）
// 输出：返回 {"ok": true} JSON 响应；同时删除会话 Cookie
// 说明：注销当前用户会话，同时清除浏览器中的 Cookie
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
    // 仅允许 POST 请求，其他方法返回 404
    if r.Method != http.MethodPost {
        http.NotFound(w, r)
        return
    }

    // 从请求 Cookie 中读取会话 ID
    // 如果 Cookie 不存在，cookie 会为 nil
    cookie, _ := r.Cookie("vt_session")
    if cookie != nil {
        // 如果会话 Cookie 存在，则调用 auth.Logout() 在服务器端删除会话
        // 注意：使用 _ 忽略错误，因为即使删除失败也需要返回 OK 并清除 Cookie
        _ = s.auth.Logout(cookie.Value)
    }

    // 删除客户端的会话 Cookie
    // 通过设置 MaxAge=-1，浏览器会立即删除此 Cookie
    http.SetCookie(w, &http.Cookie{
        // Cookie 名称必须与设置时相同
        Name: "vt_session",
        // 设置空值
        Value: "",
        // 设置 Path=/，与之前的 Cookie 路径一致
        Path: "/",
        // MaxAge=-1 表示立即删除此 Cookie（浏览器会删除）
        MaxAge: -1,
        // HttpOnly 必须与之前的设置一致
        HttpOnly: true,
    })

    // 登出成功，返回 OK 响应
    writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// (s *Server) handleCertificates 处理证书列表查询和创建请求
// 输入：GET 时无请求体，POST 时 JSON：{"domains": "example.com,www.example.com", "email": "admin@example.com"}
// 输出：GET 返回证书列表 {"items": [...]}，POST 返回新创建证书对象；都需要身份验证
// 说明：需要有效会话 Cookie（通过 withAuth() 中间件检查）
func (s *Server) handleCertificates(w http.ResponseWriter, r *http.Request) {
    // 根据 HTTP 方法类型分别处理 GET 和 POST 请求
    switch r.Method {
    // 处理 GET /api/certificates 请求 - 返回所有证书列表
    case http.MethodGet:
        // 调用证书服务获取所有证书列表
        // 返回类型：[]*certificate.Item（包含证书基本信息）
        items, err := s.certs.List()
        if err != nil {
            // 如果获取证书列表失败（数据库错误等），返回错误
            writeError(w, err)
            return
        }
        // 返回证书列表，封装在 items 字段中
        writeJSON(w, http.StatusOK, map[string]any{"items": items})

    // 处理 POST /api/certificates 请求 - 创建新证书
    case http.MethodPost:
        // 定义证书创建请求的数据结构
        var req struct {
            // 域名列表，以逗号分隔（例：example.com,www.example.com）
            Domains string `json:"domains"`
            // 申请人邮箱，用于 ACME 证书申请
            Email string `json:"email"`
        }

        // 从请求体解析 JSON
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            // 如果 JSON 格式错误，返回 400 错误
            writeError(w, fmt.Errorf("请求格式错误"))
            return
        }

        // 调用证书服务创建新证书
        // certs.Create() 会：
        // 1. 解析和验证域名列表
        // 2. 保存到数据库
        // 3. 启动 ACME 申请流程（异步）
        // 4. 返回证书对象（可能处于 pending 状态）
        item, err := s.certs.Create(req.Domains, req.Email)
        if err != nil {
            // 如果创建失败（域名格式错误、数据库错误等），返回错误
            writeError(w, err)
            return
        }

        // 返回新建证书对象
        writeJSON(w, http.StatusOK, item)

    // 其他 HTTP 方法不支持
    default:
        http.NotFound(w, r)
    }
}

// (s *Server) handleCertificateDownload 处理证书下载请求
// 输入：GET /api/certificates/{id}/download（其中 {id} 为证书 ID）
// 输出：返回 ZIP 格式的证书文件包（包含证书、密钥等）；需要身份验证
// 说明：返回 ZIP 格式以打包多个证书文件（fullchain.pem、privkey.pem 等）
func (s *Server) handleCertificateDownload(w http.ResponseWriter, r *http.Request) {
    // 仅允许 GET 请求，其他方法返回 404
    if r.Method != http.MethodGet {
        http.NotFound(w, r)
        return
    }

    // 提取 URL 路径中的证书 ID
    // 完整路径：/api/certificates/{id}/download
    // 移除前缀 /api/certificates/ 后得到：{id}/download
    path := strings.TrimPrefix(r.URL.Path, "/api/certificates/")

    // 验证路径确实以 /download 结尾
    // 这样可以区分 /api/certificates/{id}/download 和其他不合法的路径
    if !strings.HasSuffix(path, "/download") {
        // 如果路径格式错误，返回 404
        http.NotFound(w, r)
        return
    }

    // 移除末尾的 /download 后缀，提取证书 ID
    // 例如：从 "123/download" 得到 "123"
    idText := strings.TrimSuffix(path, "/download")
    // 移除前后空格（如果有）
    idText = strings.Trim(idText, "/")

    // 将字符串类型的 ID 转换为 int64 整数
    // ParseInt(string, base=10, bitSize=64) - 按十进制解析，转为 64 位整数
    id, err := strconv.ParseInt(idText, 10, 64)
    if err != nil {
        // 如果 ID 不是有效的数字，返回错误
        writeError(w, fmt.Errorf("证书ID无效"))
        return
    }

    // 调用证书服务构建 ZIP 包
    // certs.BuildZip() 会：
    // 1. 从数据库查询指定 ID 的证书
    // 2. 检查证书是否已签发（status=issued）
    // 3. 从磁盘读取证书文件和私钥文件
    // 4. 将所有证书文件打包成一个 ZIP 文件
    // 返回：ZIP 文件内容、建议的文件名、错误信息
    content, filename, err := s.certs.BuildZip(id)
    if err != nil {
        // 如果构建 ZIP 失败（证书不存在、未签发、文件读取错误等），返回错误
        writeError(w, err)
        return
    }

    // 设置 HTTP 响应头，告知浏览器这是一个 ZIP 文件
    // Content-Type: application/zip 表示文件类型是 ZIP 压缩包
    w.Header().Set("Content-Type", "application/zip")
    // 设置 Content-Disposition 头，告知浏览器下载文件而不是在页面中显示
    // filename="..." 指定建议的下载文件名
    w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
    // 设置 HTTP 状态码为 200 OK
    w.WriteHeader(http.StatusOK)
    // 将 ZIP 文件内容写入响应体
    // 注意：使用 _ 忽略写入的字节数，因为在这里不需要关心
    _, _ = w.Write(content)
}

// (s *Server) withAuth 是 HTTP 中间件，用于检查用户会话的有效性
// 输入：next = 下一个处理函数（如 handleCertificates 等）
// 输出：返回一个 HTTP 处理函数，会先检查身份验证再调用 next
// 说明：实现了简单的会话验证逻辑，保护需要身份验证的 API 端点
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
    // 返回一个新的 HTTP 处理函数，包装了原始的 next 函数
    return func(w http.ResponseWriter, r *http.Request) {
        // 检查当前请求路径是否是公开接口（不需要身份验证）
        // 这三个接口必须公开，否则无法进行初始化和登录
        if r.URL.Path == "/api/bootstrap" || r.URL.Path == "/api/bootstrap/admin" || r.URL.Path == "/api/login" {
            // 如果是公开接口，直接调用 next 处理函数，跳过身份验证
            next(w, r)
            return
        }

        // 从请求 Cookie 中读取会话 ID
        cookie, err := r.Cookie("vt_session")
        if err != nil {
            // 如果 Cookie 不存在或读取失败，说明用户未登录，返回 401 未授权错误
            writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "请先登录"})
            return
        }

        // 验证会话 ID 是否有效
        // auth.ValidateSession() 会：
        // 1. 查询数据库检查会话是否存在
        // 2. 检查会话是否过期
        // 3. 返回会话有效性（ok）和可能的错误（err）
        ok, err := s.auth.ValidateSession(cookie.Value)
        if err != nil || !ok {
            // 如果会话不存在、已过期或验证失败，返回 401 未授权错误
            writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "会话已失效，请重新登录"})
            return
        }

        // 会话有效，继续调用实际的处理函数
        next(w, r)
    }
}

// writeJSON 是工具函数，用于发送 JSON 格式的 HTTP 响应
// 输入：w = HTTP 响应写入器、code = HTTP 状态码（如 200、400、500）、payload = 要序列化为 JSON 的数据
// 输出：将 JSON 数据写入 HTTP 响应
// 说明：统一处理所有 JSON 响应的格式和编码，避免代码重复
func writeJSON(w http.ResponseWriter, code int, payload any) {
    // 设置 Content-Type 响应头，告知客户端这是 JSON 格式的数据
    // charset=utf-8 表示字符编码为 UTF-8（支持中文等非 ASCII 字符）
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    // 发送 HTTP 状态码（如 200、400、401、500 等）
    // 注意：必须在写入响应体之前调用，因为确定了状态码后无法更改
    w.WriteHeader(code)
    // 使用 JSON 编码器将数据对象序列化为 JSON 格式并写入响应体
    // json.NewEncoder(w).Encode(payload) 会：
    // 1. 将 payload 对象转换为 JSON 格式
    // 2. 自动处理 struct 标签（如 json:"fieldname"）
    // 3. 将 JSON 写入 ResponseWriter
    // 注意：使用 _ 忽略错误，因为在这里不需要处理写入失败的情况
    _ = json.NewEncoder(w).Encode(payload)
}

// writeError 是工具函数，用于发送 JSON 格式的错误响应
// 输入：w = HTTP 响应写入器、err = 错误对象
// 输出：将错误信息以 JSON 格式写入 HTTP 响应（状态码 400 Bad Request）
// 说明：统一处理所有错误响应的格式，简化错误处理代码
func writeError(w http.ResponseWriter, err error) {
    // 调用 writeJSON() 函数发送错误响应
    // 参数说明：
    // - w: HTTP 响应写入器
    // - http.StatusBadRequest: HTTP 状态码 400（表示客户端请求错误）
    // - 数据对象：包含 "error" 字段的 map，值为错误消息字符串
    // 用户接收的 JSON 格式：{"error": "错误信息描述"}
    writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
}
