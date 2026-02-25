package monitoring

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "sync"
    "time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
    Info     AlertLevel = "info"
    Warning  AlertLevel = "warning"
    Error    AlertLevel = "error"
    Critical AlertLevel = "critical"
)

// Alert 告警结构
type Alert struct {
    ID         string                 `json:"id"`
    Level      AlertLevel             `json:"level"`
    Title      string                 `json:"title"`
    Message    string                 `json:"message"`
    Symbol     string                 `json:"symbol,omitempty"`
    Value      float64                `json:"value,omitempty"`
    Threshold  float64                `json:"threshold,omitempty"`
    Source     string                 `json:"source"`
    Timestamp  time.Time              `json:"timestamp"`
    Resolved   bool                   `json:"resolved"`
    ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AlertChannel 告警渠道配置
type AlertChannel struct {
    Type      string                 `json:"type"` // email, feishu, dingding
    Enabled   bool                   `json:"enabled"`
    Settings  map[string]interface{} `json:"settings"`
    Filters   []AlertFilter          `json:"filters"`
    RateLimit RateLimit              `json:"rate_limit"`
}

// AlertFilter 告警过滤规则
type AlertFilter struct {
    Field    string     `json:"field"`
    Operator string     `json:"operator"` // equals, contains, gt, lt
    Value    string     `json:"value"`
    Level    AlertLevel `json:"level,omitempty"`
}

// RateLimit 限流配置
type RateLimit struct {
    MaxPerHour int           `json:"max_per_hour"`
    MaxPerDay  int           `json:"max_per_day"`
    Cooldown   time.Duration `json:"cooldown"`
}

// AlertSystem 告警系统
type AlertSystem struct {
    mu         sync.RWMutex
    alerts     map[string]*Alert        // 告警ID -> 告警
    channels   map[string]*AlertChannel // 渠道名称 -> 渠道配置
    httpClient *http.Client
    templates  map[string]string      // 模板名称 -> 模板内容
    rateLimits map[string]RateTracker // 限流追踪
    stats      *AlertStats
}

// AlertStats 告警统计
type AlertStats struct {
    TotalAlerts    int64                `json:"total_alerts"`
    ActiveAlerts   int64                `json:"active_alerts"`
    ResolvedAlerts int64                `json:"resolved_alerts"`
    ByLevel        map[AlertLevel]int64 `json:"by_level"`
    ByChannel      map[string]int64     `json:"by_channel"`
    LastAlert      time.Time            `json:"last_alert"`
    Uptime         time.Duration        `json:"uptime"`
}

// RateTracker 限流追踪器
type RateTracker struct {
    hourCount int
    dayCount  int
    lastSent  time.Time
    hourReset time.Time
    dayReset  time.Time
}

// NewAlertSystem 创建告警系统
func NewAlertSystem() *AlertSystem {
    system := &AlertSystem{
        alerts:     make(map[string]*Alert),
        channels:   make(map[string]*AlertChannel),
        httpClient: &http.Client{Timeout: 30 * time.Second},
        templates:  make(map[string]string),
        rateLimits: make(map[string]RateTracker),
        stats: &AlertStats{
            ByLevel:   make(map[AlertLevel]int64),
            ByChannel: make(map[string]int64),
        },
    }

    system.initDefaultTemplates()
    system.initDefaultChannels()

    return system
}

// Start 启动告警系统
func (a *AlertSystem) Start() error {
    // 初始化默认配置
    log.Printf("Alert system started")
    return nil
}

// Stop 停止告警系统
func (a *AlertSystem) Stop() error {
    // 清理资源
    log.Printf("Alert system stopped")
    return nil
}

// SendAlert 发送告警
func (a *AlertSystem) SendAlert(alert *Alert) error {
    if alert == nil {
        return fmt.Errorf("alert is nil")
    }

    if alert.ID == "" {
        alert.ID = generateAlertID()
    }
    if alert.Timestamp.IsZero() {
        alert.Timestamp = time.Now()
    }

    a.mu.Lock()
    a.alerts[alert.ID] = alert
    a.mu.Unlock()

    // 更新统计
    a.updateStats(alert)

    // 检查是否需要发送
    if !a.shouldSendAlert(alert) {
        log.Printf("Alert %s filtered or rate limited", alert.ID)
        return nil
    }

    // 发送到各个渠道
    return a.broadcastAlert(alert)
}

// shouldSendAlert 检查是否应该发送告警
func (a *AlertSystem) shouldSendAlert(alert *Alert) bool {
    // 检查过滤器
    for _, channel := range a.channels {
        if !channel.Enabled {
            continue
        }

        for _, filter := range channel.Filters {
            if a.matchesFilter(alert, filter) {
                // 检查限流
                if !a.checkRateLimit(channel.Type) {
                    return false
                }
                return true
            }
        }
    }

    return false
}

// matchesFilter 检查告警是否匹配过滤器
func (a *AlertSystem) matchesFilter(alert *Alert, filter AlertFilter) bool {
    var fieldValue string

    switch filter.Field {
    case "level":
        fieldValue = string(alert.Level)
    case "symbol":
        fieldValue = alert.Symbol
    case "source":
        fieldValue = alert.Source
    case "message":
        fieldValue = alert.Message
    default:
        if val, ok := alert.Metadata[filter.Field]; ok {
            fieldValue = fmt.Sprintf("%v", val)
        }
    }

    switch filter.Operator {
    case "equals":
        return fieldValue == filter.Value
    case "contains":
        return strings.Contains(fieldValue, filter.Value)
    case "gt":
        return alert.Value > alert.Threshold
    case "lt":
        return alert.Value < alert.Threshold
    }

    return false
}

// checkRateLimit 检查限流
func (a *AlertSystem) checkRateLimit(channelType string) bool {
    tracker, exists := a.rateLimits[channelType]
    if !exists {
        tracker = RateTracker{
            hourReset: time.Now().Truncate(time.Hour),
            dayReset:  time.Now().Truncate(24 * time.Hour),
        }
        a.rateLimits[channelType] = tracker
    }

    now := time.Now()

    // 重置计数器
    if now.Hour() != tracker.hourReset.Hour() {
        tracker.hourCount = 0
        tracker.hourReset = now.Truncate(time.Hour)
    }
    if now.Day() != tracker.dayReset.Day() {
        tracker.dayCount = 0
        tracker.dayReset = now.Truncate(24 * time.Hour)
    }

    // 检查限流
    channel := a.channels[channelType]
    if channel.RateLimit.MaxPerHour > 0 && tracker.hourCount >= channel.RateLimit.MaxPerHour {
        return false
    }
    if channel.RateLimit.MaxPerDay > 0 && tracker.dayCount >= channel.RateLimit.MaxPerDay {
        return false
    }

    // 检查冷却时间
    if channel.RateLimit.Cooldown > 0 && now.Sub(tracker.lastSent) < channel.RateLimit.Cooldown {
        return false
    }

    // 更新计数器
    tracker.hourCount++
    tracker.dayCount++
    tracker.lastSent = now
    a.rateLimits[channelType] = tracker

    return true
}

// broadcastAlert 广播告警到所有渠道
func (a *AlertSystem) broadcastAlert(alert *Alert) error {
    var errors []string

    for channelName, channel := range a.channels {
        if !channel.Enabled {
            continue
        }

        switch channel.Type {
        case "email":
            if err := a.sendEmailAlert(channel, alert); err != nil {
                errors = append(errors, fmt.Sprintf("email failed: %v", err))
            } else {
                a.stats.ByChannel[channelName]++
            }
        case "feishu":
            if err := a.sendFeishuAlert(channel, alert); err != nil {
                errors = append(errors, fmt.Sprintf("feishu failed: %v", err))
            } else {
                a.stats.ByChannel[channelName]++
            }
        case "dingding":
            if err := a.sendDingdingAlert(channel, alert); err != nil {
                errors = append(errors, fmt.Sprintf("dingding failed: %v", err))
            } else {
                a.stats.ByChannel[channelName]++
            }
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("some channels failed: %s", strings.Join(errors, "; "))
    }

    log.Printf("Alert %s broadcasted successfully", alert.ID)
    return nil
}

// sendEmailAlert 发送邮件告警
func (a *AlertSystem) sendEmailAlert(channel *AlertChannel, alert *Alert) error {
    // 简化的邮件发送实现
    // 在实际环境中，需要集成真实的邮件服务

    template := a.getTemplate("email")
    subject := a.formatTemplate(template, alert)

    log.Printf("EMAIL ALERT - %s: %s", alert.Level, subject)

    // 这里应该集成真实的邮件服务
    // 例如: AWS SES, SendGrid, 或其他SMTP服务

    return nil
}

// sendFeishuAlert 发送飞书告警
func (a *AlertSystem) sendFeishuAlert(channel *AlertChannel, alert *Alert) error {
    webhook, ok := channel.Settings["webhook"].(string)
    if !ok || webhook == "" {
        return fmt.Errorf("feishu webhook not configured")
    }

    // 构建飞书消息
    message := map[string]interface{}{
        "msg_type": "text",
        "content": map[string]interface{}{
            "text": fmt.Sprintf("🚨 告警通知\n\n级别: %s\n标题: %s\n内容: %s\n时间: %s\n股票: %s",
                alert.Level, alert.Title, alert.Message,
                alert.Timestamp.Format("2006-01-02 15:04:05"), alert.Symbol),
        },
    }

    return a.sendWebhookRequest(webhook, message)
}

// sendDingdingAlert 发送钉钉告警
func (a *AlertSystem) sendDingdingAlert(channel *AlertChannel, alert *Alert) error {
    webhook, ok := channel.Settings["webhook"].(string)
    if !ok || webhook == "" {
        return fmt.Errorf("dingding webhook not configured")
    }

    // 构建钉钉消息
    message := map[string]interface{}{
        "msgtype": "text",
        "text": map[string]interface{}{
            "content": fmt.Sprintf("🚨 告警通知\n\n级别: %s\n标题: %s\n内容: %s\n时间: %s\n股票: %s",
                alert.Level, alert.Title, alert.Message,
                alert.Timestamp.Format("2006-01-02 15:04:05"), alert.Symbol),
        },
    }

    return a.sendWebhookRequest(webhook, message)
}

// sendWebhookRequest 发送Webhook请求
func (a *AlertSystem) sendWebhookRequest(url string, payload interface{}) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %v", err)
    }

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

// getTemplate 获取模板
func (a *AlertSystem) getTemplate(name string) string {
    if template, ok := a.templates[name]; ok {
        return template
    }
    return "Alert: {{.Level}} - {{.Title}}\n{{.Message}}\n{{.Timestamp}}"
}

// formatTemplate 格式化模板
func (a *AlertSystem) formatTemplate(template string, alert *Alert) string {
    // 简单的模板格式化
    // 在实际环境中可以使用Go的text/template包

    result := template
    result = strings.ReplaceAll(result, "{{.Level}}", string(alert.Level))
    result = strings.ReplaceAll(result, "{{.Title}}", alert.Title)
    result = strings.ReplaceAll(result, "{{.Message}}", alert.Message)
    result = strings.ReplaceAll(result, "{{.Symbol}}", alert.Symbol)
    result = strings.ReplaceAll(result, "{{.Timestamp}}", alert.Timestamp.Format("2006-01-02 15:04:05"))

    return result
}

// AddChannel 添加告警渠道
func (a *AlertSystem) AddChannel(name string, channel *AlertChannel) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if channel.Type == "" {
        return fmt.Errorf("channel type cannot be empty")
    }

    a.channels[name] = channel
    log.Printf("Added alert channel: %s (%s)", name, channel.Type)
    return nil
}

// RemoveChannel 移除告警渠道
func (a *AlertSystem) RemoveChannel(name string) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if _, exists := a.channels[name]; !exists {
        return fmt.Errorf("channel %s not found", name)
    }

    delete(a.channels, name)
    log.Printf("Removed alert channel: %s", name)
    return nil
}

// GetAlert 获取告警
func (a *AlertSystem) GetAlert(id string) (*Alert, bool) {
    a.mu.RLock()
    defer a.mu.RUnlock()

    alert, exists := a.alerts[id]
    return alert, exists
}

// GetAllAlerts 获取所有告警
func (a *AlertSystem) GetAllAlerts() map[string]*Alert {
    a.mu.RLock()
    defer a.mu.RUnlock()

    result := make(map[string]*Alert)
    for id, alert := range a.alerts {
        result[id] = alert
    }
    return result
}

// GetActiveAlerts 获取活跃告警
func (a *AlertSystem) GetActiveAlerts() []*Alert {
    a.mu.RLock()
    defer a.mu.RUnlock()

    var active []*Alert
    for _, alert := range a.alerts {
        if !alert.Resolved {
            active = append(active, alert)
        }
    }
    return active
}

// ResolveAlert 解决告警
func (a *AlertSystem) ResolveAlert(id string) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    alert, exists := a.alerts[id]
    if !exists {
        return fmt.Errorf("alert %s not found", id)
    }

    alert.Resolved = true
    now := time.Now()
    alert.ResolvedAt = &now

    log.Printf("Alert %s resolved", id)
    return nil
}

// GetStats 获取统计信息
func (a *AlertSystem) GetStats() *AlertStats {
    a.mu.RLock()
    defer a.mu.RUnlock()

    stats := *a.stats
    stats.ActiveAlerts = int64(len(a.GetActiveAlerts()))
    stats.LastAlert = time.Now()

    return &stats
}

// updateStats 更新统计信息
func (a *AlertSystem) updateStats(alert *Alert) {
    a.stats.TotalAlerts++
    if !alert.Resolved {
        // 活跃告警计数已在GetActiveAlerts中计算
    } else {
        a.stats.ResolvedAlerts++
    }
    a.stats.ByLevel[alert.Level]++
    a.stats.LastAlert = alert.Timestamp
}

// initDefaultTemplates 初始化默认模板
func (a *AlertSystem) initDefaultTemplates() {
    a.templates = map[string]string{
        "email": `CloudQuantBot 告警通知

级别: {{.Level}}
标题: {{.Title}}
消息: {{.Message}}
时间: {{.Timestamp}}
{{if .Symbol}}股票: {{.Symbol}}{{end}}

请及时处理相关问题。`,

        "feishu": `🚨 CloudQuantBot 告警

级别: {{.Level}}
标题: {{.Title}}
消息: {{.Message}}
时间: {{.Timestamp}}
{{if .Symbol}}股票: {{.Symbol}}{{end}}`,

        "dingding": `🚨 CloudQuantBot 告警

级别: {{.Level}}
标题: {{.Title}}
消息: {{.Message}}
时间: {{.Timestamp}}
{{if .Symbol}}股票: {{.Symbol}}{{end}}`,
    }
}

// initDefaultChannels 初始化默认渠道
func (a *AlertSystem) initDefaultChannels() {
    // 飞书渠道（默认关闭）
    a.channels["feishu"] = &AlertChannel{
        Type:    "feishu",
        Enabled: false,
        Settings: map[string]interface{}{
            "webhook": "",
        },
        Filters: []AlertFilter{
            {Field: "level", Operator: "equals", Value: "error"},
            {Field: "level", Operator: "equals", Value: "critical"},
        },
        RateLimit: RateLimit{
            MaxPerHour: 10,
            MaxPerDay:  100,
            Cooldown:   5 * time.Minute,
        },
    }

    // 钉钉渠道（默认关闭）
    a.channels["dingding"] = &AlertChannel{
        Type:    "dingding",
        Enabled: false,
        Settings: map[string]interface{}{
            "webhook": "",
        },
        Filters: []AlertFilter{
            {Field: "level", Operator: "equals", Value: "warning"},
            {Field: "level", Operator: "equals", Value: "error"},
            {Field: "level", Operator: "equals", Value: "critical"},
        },
        RateLimit: RateLimit{
            MaxPerHour: 20,
            MaxPerDay:  200,
            Cooldown:   3 * time.Minute,
        },
    }

    // 邮件渠道（预留）
    a.channels["email"] = &AlertChannel{
        Type:    "email",
        Enabled: false,
        Settings: map[string]interface{}{
            "smtp_host": "",
            "smtp_port": 587,
            "username":  "",
            "password":  "",
            "to":        "",
        },
        Filters: []AlertFilter{
            {Field: "level", Operator: "equals", Value: "critical"},
        },
        RateLimit: RateLimit{
            MaxPerHour: 5,
            MaxPerDay:  50,
            Cooldown:   30 * time.Minute,
        },
    }
}

// 工具函数
func generateAlertID() string {
    return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}
