package realtime

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
)

// AlertType 告警类型
type AlertType string

const (
    AlertTypeEmail   AlertType = "email"
    AlertTypeWebhook AlertType = "webhook"
    AlertTypeLog     AlertType = "log"
)

// AlertConfig 告警配置
type AlertConfig struct {
    Type      AlertType         `json:"type"`
    Enabled   bool              `json:"enabled"`
    RateLimit *RateLimitConfig  `json:"rate_limit"`
    Settings  map[string]string `json:"settings"`
    Filter    *AlertFilter      `json:"filter"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
    MaxPerHour int           `json:"max_per_hour"`
    MaxPerDay  int           `json:"max_per_day"`
    Cooldown   time.Duration `json:"cooldown"`
}

// AlertFilter 告警过滤器
type AlertFilter struct {
    MinLevel RiskLevel `json:"min_level"`
    Types    []string  `json:"types"`
    Symbols  []string  `json:"symbols"`
}

// RateLimiter 限流器
type RateLimiter struct {
    hourlyCount int
    dailyCount  int
    lastAlert   time.Time
    hourStart   time.Time
    dayStart    time.Time
    lock        sync.Mutex
}

// AlertTrigger 告警触发器
type AlertTrigger struct {
    configs    map[string]*AlertConfig
    configLock sync.RWMutex

    rateLimiters map[string]*RateLimiter
    limiterLock  sync.RWMutex

    httpClient *http.Client
}

// NewAlertTrigger 创建告警触发器
func NewAlertTrigger() *AlertTrigger {
    return &AlertTrigger{
        configs:      make(map[string]*AlertConfig),
        rateLimiters: make(map[string]*RateLimiter),
        httpClient:   &http.Client{Timeout: 30 * time.Second},
    }
}

// AddAlertConfig 添加告警配置
func (t *AlertTrigger) AddAlertConfig(name string, config *AlertConfig) error {
    t.configLock.Lock()
    defer t.configLock.Unlock()

    if _, ok := t.configs[name]; ok {
        return fmt.Errorf("alert config %s already exists", name)
    }

    t.configs[name] = config

    // 初始化限流器
    if config.RateLimit != nil {
        t.limiterLock.Lock()
        t.rateLimiters[name] = &RateLimiter{
            hourStart: time.Now(),
            dayStart:  time.Now(),
        }
        t.limiterLock.Unlock()
    }

    log.Printf("Added alert config: %s (type: %s, enabled: %v)", name, config.Type, config.Enabled)
    return nil
}

// UpdateAlertConfig 更新告警配置
func (t *AlertTrigger) UpdateAlertConfig(name string, config *AlertConfig) error {
    t.configLock.Lock()
    defer t.configLock.Unlock()

    if _, ok := t.configs[name]; !ok {
        return fmt.Errorf("alert config %s not found", name)
    }

    t.configs[name] = config
    log.Printf("Updated alert config: %s", name)
    return nil
}

// RemoveAlertConfig 删除告警配置
func (t *AlertTrigger) RemoveAlertConfig(name string) error {
    t.configLock.Lock()
    defer t.configLock.Unlock()

    if _, ok := t.configs[name]; !ok {
        return fmt.Errorf("alert config %s not found", name)
    }

    delete(t.configs, name)

    t.limiterLock.Lock()
    delete(t.rateLimiters, name)
    t.limiterLock.Unlock()

    log.Printf("Removed alert config: %s", name)
    return nil
}

// Trigger 触发告警
func (t *AlertTrigger) Trigger(ctx context.Context, event RiskEvent) error {
    t.configLock.RLock()
    defer t.configLock.RUnlock()

    var errors []error

    for name, config := range t.configs {
        if !config.Enabled {
            continue
        }

        // 检查过滤器
        if !t.matchFilter(event, config.Filter) {
            continue
        }

        // 检查限流
        if !t.checkRateLimit(name, config) {
            continue
        }

        // 发送告警
        if err := t.sendAlert(ctx, name, config, event); err != nil {
            log.Printf("Failed to send alert %s: %v", name, err)
            errors = append(errors, err)
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("some alerts failed: %v", errors)
    }

    return nil
}

// matchFilter 检查过滤器
func (t *AlertTrigger) matchFilter(event RiskEvent, filter *AlertFilter) bool {
    if filter == nil {
        return true
    }

    // 检查级别
    if event.Level < filter.MinLevel {
        return false
    }

    // 检查类型
    if len(filter.Types) > 0 {
        found := false
        for _, allowedType := range filter.Types {
            if event.Type == allowedType {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }

    // 检查标的
    if len(filter.Symbols) > 0 && event.Symbol != "" {
        found := false
        for _, allowedSymbol := range filter.Symbols {
            if event.Symbol == allowedSymbol {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }

    return true
}

// checkRateLimit 检查限流
func (t *AlertTrigger) checkRateLimit(name string, config *AlertConfig) bool {
    if config.RateLimit == nil {
        return true
    }

    t.limiterLock.Lock()
    defer t.limiterLock.Unlock()

    limiter, ok := t.rateLimiters[name]
    if !ok {
        return true
    }

    limiter.lock.Lock()
    defer limiter.lock.Unlock()

    now := time.Now()

    // 重置小时计数
    if now.Sub(limiter.hourStart) >= time.Hour {
        limiter.hourlyCount = 0
        limiter.hourStart = now
    }

    // 重置日计数
    if now.Sub(limiter.dayStart) >= 24*time.Hour {
        limiter.dailyCount = 0
        limiter.dayStart = now
    }

    // 检查冷却时间
    if now.Sub(limiter.lastAlert) < config.RateLimit.Cooldown {
        return false
    }

    // 检查小时限制
    if limiter.hourlyCount >= config.RateLimit.MaxPerHour {
        return false
    }

    // 检查日限制
    if limiter.dailyCount >= config.RateLimit.MaxPerDay {
        return false
    }

    // 更新计数
    limiter.hourlyCount++
    limiter.dailyCount++
    limiter.lastAlert = now

    return true
}

// sendAlert 发送告警
func (t *AlertTrigger) sendAlert(ctx context.Context, name string, config *AlertConfig, event RiskEvent) error {
    switch config.Type {
    case AlertTypeWebhook:
        return t.sendWebhook(ctx, config.Settings, event)
    case AlertTypeLog:
        return t.sendLog(event)
    case AlertTypeEmail:
        return t.sendEmail(config.Settings, event)
    default:
        return fmt.Errorf("unsupported alert type: %s", config.Type)
    }
}

// sendWebhook 发送Webhook
func (t *AlertTrigger) sendWebhook(ctx context.Context, settings map[string]string, event RiskEvent) error {
    url, ok := settings["url"]
    if !ok {
        return fmt.Errorf("webhook url not configured")
    }

    data, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "CloudQuantBot/1.0")

    resp, err := t.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send webhook: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }

    return nil
}

// sendLog 发送日志
func (t *AlertTrigger) sendLog(event RiskEvent) error {
    eventJSON, err := event.ToJSON()
    if err != nil {
        return err
    }

    switch event.Level {
    case RiskLevelCritical:
        log.Printf("[CRITICAL] %s", eventJSON)
    case RiskLevelHigh:
        log.Printf("[WARNING] %s", eventJSON)
    case RiskLevelMedium:
        log.Printf("[INFO] %s", eventJSON)
    default:
        log.Printf("[DEBUG] %s", eventJSON)
    }

    return nil
}

// sendEmail 发送邮件（占位符）
func (t *AlertTrigger) sendEmail(settings map[string]string, event RiskEvent) error {
    // 这里需要实现邮件发送逻辑
    log.Printf("Email alert not implemented for event: %s", event.ID)
    return nil
}

// GetRateLimitStatus 获取限流状态
func (t *AlertTrigger) GetRateLimitStatus(name string) map[string]interface{} {
    t.limiterLock.RLock()
    defer t.limiterLock.RUnlock()

    limiter, ok := t.rateLimiters[name]
    if !ok {
        return nil
    }

    return map[string]interface{}{
        "hourly_count": limiter.hourlyCount,
        "daily_count":  limiter.dailyCount,
        "last_alert":   limiter.lastAlert,
        "hour_start":   limiter.hourStart,
        "day_start":    limiter.dayStart,
    }
}
