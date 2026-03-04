// 文件：internal/service/renewal.go | 用途：自动续期后台任务 | 关键对象：RenewalWorker
// 包：service | 用途：业务逻辑层，负责证书自动续期调度 | 关键接口/结构：RenewalWorker
package service

import (
    // context 包用于控制后台任务生命周期
    // 用途：接收取消信号，优雅停止续期循环
    "context"
    // log 包用于输出运行日志
    // 用途：记录自动续期失败原因，便于排障
    "log"
    // time 包用于定时调度
    // 用途：Ticker 周期触发续期检查
    "time"
    // config 包提供自动续期配置
    // 用途：读取启用开关、检查间隔、提前续期天数
    "vt-cert-panel/internal/config"
)

// RenewalWorker 表示自动续期工作器 | 关键字段：cfg（续期配置）、service（证书业务服务）
type RenewalWorker struct {
    // cfg 表示全局配置对象 | 类型：*config.Config | 作用域：续期参数读取
    cfg     *config.Config
    // service 表示证书业务服务 | 类型：*CertificateService | 作用域：执行实际续期逻辑
    service *CertificateService
}

// NewRenewalWorker 用于创建自动续期工作器 | 输入：cfg 配置、service 证书服务 | 输出：*RenewalWorker | 可能失败：无
func NewRenewalWorker(cfg *config.Config, service *CertificateService) *RenewalWorker {
    // 构造并返回工作器实例，完成依赖注入
    return &RenewalWorker{cfg: cfg, service: service}
}

// Start 用于启动自动续期循环 | 输入：ctx（用于取消） | 输出：无 | 可能失败：无（错误在 runOnce 内记录日志）
func (w *RenewalWorker) Start(ctx context.Context) {
    // 如果配置中关闭了自动续期，则直接退出，不启动任何后台循环
    if !w.cfg.AutoRenewal.Enabled {
        return
    }

    // 创建周期性 ticker：每 CheckIntervalHour 小时触发一次续期检查
    // 启动 goroutine 用于定时执行续期逻辑，超时由 ticker 控制，退出由 ctx.Done() 控制
    ticker := time.NewTicker(time.Duration(w.cfg.AutoRenewal.CheckIntervalHour) * time.Hour)
    // 函数退出时停止 ticker，释放底层计时器资源，避免泄漏
    defer ticker.Stop()

    // 启动后立即执行一次，避免首次检查要等待一个完整周期
    w.runOnce()

    // 进入常驻循环：等待取消信号或定时信号
    for {
        select {
        case <-ctx.Done():
            // 收到取消信号（如应用关闭）时，优雅退出循环
            return
        case <-ticker.C:
            // 到达定时点，执行一次续期检查
            w.runOnce()
        }
    }
}

// runOnce 用于执行一次续期检查与处理 | 输入：无 | 输出：无 | 可能失败：续期失败（仅记录日志，不中断服务）
func (w *RenewalWorker) runOnce() {
    // 调用证书服务：续期“在 RenewBeforeDays 天内到期”的证书
    if err := w.service.RenewDue(w.cfg.AutoRenewal.RenewBeforeDays); err != nil {
        // 如果续期流程失败，则记录日志用于排查；不 panic，避免后台任务整体崩溃
        log.Printf("自动续期失败: %v", err)
    }
}
