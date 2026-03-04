// 文件：main.go | 用途：应用程序入口点 | 关键函数：main
// 权限要求：普通用户权限 | 依赖工具：Go 1.23+
package main

// === 依赖包导入 === | 说明：应用程序启动所需的核心包
import (
    // log 包用于日志输出 | 用途：程序启动和错误日志 | 来自：Go 标准库
    "log"
    // app 包包含应用程序的初始化和启动逻辑 | 用途：初始化应用（app.New）、启动 HTTP 服务器和后台 worker（application.Run）
    // | 来自：内部模块 vt-cert-panel/internal/app
    "vt-cert-panel/internal/app"
)

// main 函数是程序的入口点，负责初始化应用和启动服务
// 流程：初始化应用（app.New）-> 启动应用（application.Run）-> 处理错误
func main() {
    // 初始化应用程序，包括数据库、服务、HTTP 服务器等
    application, err := app.New()
    if err != nil {
        // 初始化失败，致命错误，立即退出
        log.Fatalf("初始化失败: %v", err)
    }

    // 启动应用，监听 HTTP 请求和后台 goroutine（证书自动续期）
    // 在收到中断信号（Ctrl+C）时优雅关闭
    if err := application.Run(); err != nil {
        // 运行时发生错误，致命错误，立即退出
        log.Fatalf("运行失败: %v", err)
    }
}
