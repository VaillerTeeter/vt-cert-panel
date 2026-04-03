// 文件：go.mod | 用途：Go 项目的依赖声明与版本管理 | 关键依赖：lego（ACME 证书）、crypto（加密）、sqlite（数据库）

// 模块声明：定义当前 Go 项目的模块名称（用于导入）
module vt-cert-panel

// Go 版本：指定项目开发与编译的 Go 语言版本要求（1.23 或更高版本）
// 版本说明：1.23 是 Go 1.24 发布前的最新稳定版本，提供最新的语言特性与性能优化
go 1.24.0

// 依赖库声明：项目直接依赖的外部库（以及库的版本）
require (
	// github.com/go-acme/lego/v4 v4.20.2
	// 用途：ACME 证书自动申请与续期（支持 Let's Encrypt）
	// 功能：提供 ACME 协议客户端，支持 HTTP-01、DNS-01 等验证方式
	// 版本说明：v4.20.2 表示主版本 4（v4）、次版本 20、补丁版本 2
	// 为什么选择：业界标准的 ACME 客户端库，活跃维护，支持多种 DNS 提供商集成
	github.com/go-acme/lego/v4 v4.25.2

	// golang.org/x/crypto v0.33.0
	// 用途：密码学算法库（用于加密、哈希、随机数等）
	// 功能：提供 AES、SHA、PBKDF2 等加密算法，本项目用于密码哈希与会话 token 生成
	// 版本说明：v0.33.0 表示非主版本（版本号以 v0 开头，api 可能变更），33 是次版本，0 是补丁版本
	// 为什么选择：Go 官方维护的加密库，性能稳定，安全推荐
	golang.org/x/crypto v0.45.0

	// modernc.org/sqlite v1.34.2
	// 用途：SQLite 数据库驱动（纯 Go 实现，不依赖 C 库）
	// 功能：Go 程序与 SQLite 数据库的桥梁，支持连接管理、事务、查询等
	// 版本说明：v1.34.2 表示主版本 1、次版本 34、补丁版本 2
	// 为什么选择：纯 Go 实现无需依赖外部 C 库（cgo），编译灵活，跨平台友好，性能接近原生 SQLite
	modernc.org/sqlite v1.34.2
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.67 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	modernc.org/gc/v3 v3.0.0-20240107210532-573471604cb6 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
)
