// Package monitoring 提供策略回放功能
package monitoring

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ReplaySession 策略回放会话
type ReplaySession struct {
	ID           string         `json:"id"`
	Symbol       string         `json:"symbol"`
	StartDate    time.Time      `json:"start_date"`
	EndDate      time.Time      `json:"end_date"`
	Speed        float64        `json:"speed"`  // 回放速度(1x, 2x, 5x, 10x)
	Status       ReplayStatus   `json:"status"` // playing, paused, stopped
	CurrentTime  time.Time      `json:"current_time"`
	Progress     float64        `json:"progress"`      // 0-100%
	Signals      []ReplaySignal `json:"signals"`       // 产生的信号
	Events       []ReplayEvent  `json:"events"`        // 关键事件
	CurrentIndex int            `json:"current_index"` // 当前数据索引
	TotalData    int            `json:"total_data"`    // 总数据量
}

// ReplayStatus 回放状态
type ReplayStatus string

const (
	ReplayPlaying ReplayStatus = "playing"
	ReplayPaused  ReplayStatus = "paused"
	ReplayStopped ReplayStatus = "stopped"
)

// ReplaySignal 回放信号
type ReplaySignal struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // buy, sell
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	Strategy  string    `json:"strategy"`
	Reason    string    `json:"reason"`
}

// ReplayEvent 回放事件
type ReplayEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // signal, trade, alert
	Data      interface{} `json:"data"`
	Message   string      `json:"message"`
}

// ReplayEngine 回放引擎
type ReplayEngine struct {
	sessions     map[string]*ReplaySession
	sessionsMu   sync.RWMutex
	dataProvider ReplayDataProvider
	stopChan     map[string]chan struct{}
	pauseChan    map[string]chan struct{}
	mu           sync.Mutex
}

// ReplayDataProvider 回放数据源接口
type ReplayDataProvider interface {
	FetchData(symbol string, start, end time.Time) ([]ReplayDataPoint, error)
}

// ReplayDataPoint 回放数据点
type ReplayDataPoint struct {
	Timestamp time.Time      `json:"timestamp"`
	Open      float64        `json:"open"`
	High      float64        `json:"high"`
	Low       float64        `json:"low"`
	Close     float64        `json:"close"`
	Volume    int64          `json:"volume"`
	Signals   []ReplaySignal `json:"signals,omitempty"`
}

// NewReplayEngine 创建回放引擎
func NewReplayEngine(dataProvider ReplayDataProvider) *ReplayEngine {
	return &ReplayEngine{
		sessions:     make(map[string]*ReplaySession),
		stopChan:     make(map[string]chan struct{}),
		pauseChan:    make(map[string]chan struct{}),
		dataProvider: dataProvider,
	}
}

// StartSession 开始回放会话
func (re *ReplayEngine) StartSession(symbol string, startDate, endDate time.Time, speed float64) (*ReplaySession, error) {
	// 获取历史数据
	data, err := re.dataProvider.FetchData(symbol, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取回放数据失败: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("无回放数据")
	}

	// 创建会话
	session := &ReplaySession{
		ID:           generateSessionID(),
		Symbol:       symbol,
		StartDate:    startDate,
		EndDate:      endDate,
		Speed:        speed,
		Status:       ReplayPlaying,
		CurrentTime:  data[0].Timestamp,
		Progress:     0,
		Signals:      make([]ReplaySignal, 0),
		Events:       make([]ReplayEvent, 0),
		CurrentIndex: 0,
		TotalData:    len(data),
	}

	re.sessionsMu.Lock()
	re.sessions[session.ID] = session
	re.stopChan[session.ID] = make(chan struct{})
	re.pauseChan[session.ID] = make(chan struct{})
	re.sessionsMu.Unlock()

	// 启动回放
	go re.runReplay(session, data)

	return session, nil
}

// runReplay 运行回放
func (re *ReplayEngine) runReplay(session *ReplaySession, data []ReplayDataPoint) {
	ticker := time.NewTicker(time.Second / time.Duration(session.Speed))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if session.CurrentIndex >= len(data) {
				session.Status = ReplayStopped
				re.addEvent(session, ReplayEvent{
					Timestamp: time.Now(),
					Type:      "complete",
					Message:   "回放完成",
				})
				return
			}

			// 更新当前数据点
			point := data[session.CurrentIndex]
			session.CurrentTime = point.Timestamp
			session.CurrentIndex++
			session.Progress = float64(session.CurrentIndex) / float64(len(data)) * 100

			// 处理信号
			for _, signal := range point.Signals {
				session.Signals = append(session.Signals, signal)
				re.addEvent(session, ReplayEvent{
					Timestamp: point.Timestamp,
					Type:      "signal",
					Data:      signal,
					Message:   fmt.Sprintf("%s 信号: %s @ %.2f", signal.Type, signal.Strategy, signal.Price),
				})
			}

			// 广播当前状态
			re.broadcastState(session)

		case <-re.pauseChan[session.ID]:
			// 等待恢复
			<-re.pauseChan[session.ID]

		case <-re.stopChan[session.ID]:
			session.Status = ReplayStopped
			return
		}
	}
}

// PauseSession 暂停回放
func (re *ReplayEngine) PauseSession(sessionID string) error {
	re.sessionsMu.RLock()
	session, exists := re.sessions[sessionID]
	re.sessionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("会话不存在")
	}

	if session.Status != ReplayPlaying {
		return fmt.Errorf("会话不在播放状态")
	}

	session.Status = ReplayPaused
	re.addEvent(session, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "pause",
		Message:   "回放已暂停",
	})

	return nil
}

// ResumeSession 恢复回放
func (re *ReplayEngine) ResumeSession(sessionID string) error {
	re.sessionsMu.RLock()
	session, exists := re.sessions[sessionID]
	re.sessionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("会话不存在")
	}

	if session.Status != ReplayPaused {
		return fmt.Errorf("会话不在暂停状态")
	}

	session.Status = ReplayPlaying
	re.addEvent(session, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "resume",
		Message:   "回放已恢复",
	})

	return nil
}

// StopSession 停止回放
func (re *ReplayEngine) StopSession(sessionID string) error {
	re.sessionsMu.RLock()
	_, exists := re.sessions[sessionID]
	re.sessionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("会话不存在")
	}

	re.mu.Lock()
	if stopChan, ok := re.stopChan[sessionID]; ok {
		close(stopChan)
	}
	re.mu.Unlock()

	return nil
}

// SetSpeed 设置回放速度
func (re *ReplayEngine) SetSpeed(sessionID string, speed float64) error {
	re.sessionsMu.RLock()
	session, exists := re.sessions[sessionID]
	re.sessionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("会话不存在")
	}

	if speed <= 0 || speed > 100 {
		return fmt.Errorf("无效的速度值")
	}

	session.Speed = speed
	re.addEvent(session, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "speed",
		Data:      speed,
		Message:   fmt.Sprintf("回放速度调整为 %.1fx", speed),
	})

	return nil
}

// GetSession 获取回放会话
func (re *ReplayEngine) GetSession(sessionID string) (*ReplaySession, error) {
	re.sessionsMu.RLock()
	defer re.sessionsMu.RUnlock()

	session, exists := re.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在")
	}

	// 返回副本
	sessionCopy := *session
	return &sessionCopy, nil
}

// GetAllSessions 获取所有回放会话
func (re *ReplayEngine) GetAllSessions() []*ReplaySession {
	re.sessionsMu.RLock()
	defer re.sessionsMu.RUnlock()

	sessions := make([]*ReplaySession, 0, len(re.sessions))
	for _, session := range re.sessions {
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}

	return sessions
}

// DeleteSession 删除回放会话
func (re *ReplayEngine) DeleteSession(sessionID string) error {
	re.sessionsMu.Lock()
	defer re.sessionsMu.Unlock()

	if _, exists := re.sessions[sessionID]; !exists {
		return fmt.Errorf("会话不存在")
	}

	// 停止回放
	re.mu.Lock()
	if stopChan, ok := re.stopChan[sessionID]; ok {
		close(stopChan)
	}
	delete(re.stopChan, sessionID)
	delete(re.pauseChan, sessionID)
	re.mu.Unlock()

	delete(re.sessions, sessionID)
	return nil
}

// addEvent 添加事件
func (re *ReplayEngine) addEvent(session *ReplaySession, event ReplayEvent) {
	re.sessionsMu.Lock()
	defer re.sessionsMu.Unlock()

	session.Events = append(session.Events, event)
}

// broadcastState 广播回放状态
func (re *ReplayEngine) broadcastState(session *ReplaySession) {
	// 通过WebSocket广播
	// 使用现有的WebSocketHub进行广播
	// 这里简化处理，实际应注入WebSocketHub实例
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	return fmt.Sprintf("replay_%d", time.Now().UnixNano())
}

// ReplayConfig 回放配置
type ReplayConfig struct {
	Symbol    string    `json:"symbol"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Speed     float64   `json:"speed"`
}

// ToJSON 导出为JSON
func (rs *ReplaySession) ToJSON() ([]byte, error) {
	return json.Marshal(rs)
}

// GetEvents 获取回放事件
func (rs *ReplaySession) GetEvents(eventType string) []ReplayEvent {
	if eventType == "" {
		return rs.Events
	}

	var filtered []ReplayEvent
	for _, event := range rs.Events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// GetSignals 获取回放信号
func (rs *ReplaySession) GetSignals(signalType string) []ReplaySignal {
	if signalType == "" {
		return rs.Signals
	}

	var filtered []ReplaySignal
	for _, signal := range rs.Signals {
		if signal.Type == signalType {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// MockReplayDataProvider Mock回放数据提供者
type MockReplayDataProvider struct {
	basePrice  float64
	volatility float64
}

// NewMockReplayDataProvider 创建Mock回放数据提供者
func NewMockReplayDataProvider() *MockReplayDataProvider {
	return &MockReplayDataProvider{
		basePrice:  100.0,
		volatility: 0.02,
	}
}

// FetchData 获取Mock回放数据
func (m *MockReplayDataProvider) FetchData(symbol string, start, end time.Time) ([]ReplayDataPoint, error) {
	var data []ReplayDataPoint
	currentPrice := m.basePrice

	for current := start; current.Before(end); current = current.Add(time.Minute) {
		// 模拟价格变动
		change := (float64(current.Unix()%100) - 50) / 1000 * m.volatility
		currentPrice = currentPrice * (1 + change)

		point := ReplayDataPoint{
			Timestamp: current,
			Open:      currentPrice * 0.99,
			High:      currentPrice * 1.01,
			Low:       currentPrice * 0.98,
			Close:     currentPrice,
			Volume:    int64(current.Unix() % 1000000),
		}

		// 随机生成信号
		if current.Unix()%60 == 0 {
			signalType := "buy"
			if current.Unix()%2 == 0 {
				signalType = "sell"
			}
			point.Signals = []ReplaySignal{
				{
					Timestamp: current,
					Type:      signalType,
					Price:     currentPrice,
					Quantity:  100,
					Strategy:  "MA_Cross",
					Reason:    "Mock signal",
				},
			}
		}

		data = append(data, point)
	}

	return data, nil
}
