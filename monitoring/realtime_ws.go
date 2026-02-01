package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType 消息类型
type MessageType string

const (
	MarketData    MessageType = "market_data"
	StrategySignal MessageType = "strategy_signal"
	TradeEvent    MessageType = "trade_event"
	RiskAlert     MessageType = "risk_alert"
	SystemStatus  MessageType = "system_status"
	Heartbeat     MessageType = "heartbeat"
)

// Message 监控消息结构
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
	ID        string          `json:"id"`
}

// Client WebSocket客户端
type Client struct {
	conn         *websocket.Conn
	send         chan []byte
	clientID     string
	subscriptions map[string]bool // 订阅的消息类型
}

// WebSocketHub WebSocket中心
type WebSocketHub struct {
	clients     map[*Client]bool
	broadcast   chan []byte
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
	upgrader    websocket.Upgrader
	ctx         context.Context
	cancel      context.CancelFunc
}

// RealtimeMonitor 实时监控器
type RealtimeMonitor struct {
	hub         *WebSocketHub
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	running     bool
	stats       *MonitorStats
	alertSystem *AlertSystem
}

// MonitorStats 监控统计
type MonitorStats struct {
	ConnectedClients    int64         `json:"connected_clients"`
	MessagesSent       int64         `json:"messages_sent"`
	MessagesReceived   int64         `json:"messages_received"`
	StartTime          time.Time     `json:"start_time"`
	LastMessageTime    time.Time     `json:"last_message_time"`
	Uptime             time.Duration `json:"uptime"`
}

// NewWebSocketHub 创建WebSocket中心
func NewWebSocketHub() *WebSocketHub {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketHub{
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, 256),
		register:  make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 在生产环境中应该设置更严格的origin检查
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动WebSocket中心
func (h *WebSocketHub) Start() {
	defer func() {
		log.Printf("WebSocket hub stopped")
	}()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected: %s (total: %d)", client.clientID, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected: %s (total: %d)", client.clientID, len(h.clients))

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()

		case <-h.ctx.Done():
			// 关闭所有连接
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Stop 停止WebSocket中心
func (h *WebSocketHub) Stop() {
	h.cancel()
}

// HandleWebSocket 处理WebSocket连接
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	clientID := generateClientID()
	client := &Client{
		conn:          conn,
		send:          make(chan []byte, 256),
		clientID:      clientID,
		subscriptions: make(map[string]bool),
	}

	h.register <- client

	// 启动客户端协程
	go client.writePump()
	go client.readPump(h)
}

// Broadcast 广播消息
func (h *WebSocketHub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		log.Printf("WebSocket broadcast queue is full, dropping message")
	}
}

// SendToClient 发送消息给特定客户端
func (h *WebSocketHub) SendToClient(clientID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.clientID == clientID {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
			break
		}
	}
}

// writePump WebSocket写入泵
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump WebSocket读取泵
func (c *Client) readPump(h *WebSocketHub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	for {
		_, messageData, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 处理客户端消息
		var clientMsg ClientMessage
		if err := json.Unmarshal(messageData, &clientMsg); err != nil {
			log.Printf("Failed to parse client message: %v", err)
			continue
		}

		c.handleClientMessage(clientMsg)
	}
}

// handleClientMessage 处理客户端消息
func (c *Client) handleClientMessage(msg ClientMessage) {
	switch msg.Type {
	case "subscribe":
		c.subscriptions[msg.Topic] = true
		log.Printf("Client %s subscribed to %s", c.clientID, msg.Topic)
	case "unsubscribe":
		delete(c.subscriptions, msg.Topic)
		log.Printf("Client %s unsubscribed from %s", c.clientID, msg.Topic)
	case "ping":
		// 处理ping消息
		log.Printf("Ping from client %s", c.clientID)
	}
}

// NewRealtimeMonitor 创建实时监控器
func NewRealtimeMonitor() *RealtimeMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	monitor := &RealtimeMonitor{
		hub:     NewWebSocketHub(),
		ctx:     ctx,
		cancel:  cancel,
		stats: &MonitorStats{
			StartTime: time.Now(),
		},
	}

	return monitor
}

// Start 启动监控器
func (m *RealtimeMonitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("monitor is already running")
	}

	// 启动WebSocket中心
	go m.hub.Start()

	m.running = true
	m.stats.StartTime = time.Now()

	log.Printf("Realtime monitor started")
	return nil
}

// Stop 停止监控器
func (m *RealtimeMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	m.running = false
	m.hub.Stop()
	m.cancel()

	log.Printf("Realtime monitor stopped")
	return nil
}

// SendMarketData 发送市场数据
func (m *RealtimeMonitor) SendMarketData(data MarketDataMessage) error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	msg := Message{
		Type:      MarketData,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal market data: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	log.Printf("Sent market data for %s", data.Symbol)
	return nil
}

// SendStrategySignal 发送策略信号
func (m *RealtimeMonitor) SendStrategySignal(signal StrategySignalMessage) error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	msg := Message{
		Type:      StrategySignal,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy signal: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	log.Printf("Sent strategy signal: %s %s (strength: %.2f)", signal.Symbol, signal.SignalType, signal.Strength)
	return nil
}

// SendTradeEvent 发送交易事件
func (m *RealtimeMonitor) SendTradeEvent(event TradeEventMessage) error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	msg := Message{
		Type:      TradeEvent,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal trade event: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	log.Printf("Sent trade event: %s %s %d shares", event.Symbol, event.Action, event.Quantity)
	return nil
}

// SendRiskAlert 发送风险告警
func (m *RealtimeMonitor) SendRiskAlert(alert RiskAlertMessage) error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	msg := Message{
		Type:      RiskAlert,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal risk alert: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	log.Printf("Sent risk alert: %s - %s", alert.Level, alert.Message)
	return nil
}

// SendSystemStatus 发送系统状态
func (m *RealtimeMonitor) SendSystemStatus(status SystemStatusMessage) error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	msg := Message{
		Type:      SystemStatus,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal system status: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	return nil
}

// SendHeartbeat 发送心跳
func (m *RealtimeMonitor) SendHeartbeat() error {
	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	heartbeat := HeartbeatMessage{
		Timestamp: time.Now(),
		Status:    "alive",
	}

	msg := Message{
		Type:      Heartbeat,
		Timestamp: time.Now(),
		ID:        generateMessageID(),
	}

	msgData, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %v", err)
	}
	msg.Data = msgData

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	m.hub.Broadcast(messageBytes)
	m.updateStats(len(messageBytes), 0)

	return nil
}

// GetStats 获取监控统计
func (m *RealtimeMonitor) GetStats() *MonitorStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := *m.stats
	if m.running {
		stats.Uptime = time.Since(m.stats.StartTime)
	}
	stats.ConnectedClients = int64(len(m.hub.clients))
	stats.LastMessageTime = time.Now()

	return &stats
}

// GetWebSocketHub 获取WebSocket中心
func (m *RealtimeMonitor) GetWebSocketHub() *WebSocketHub {
	return m.hub
}

// updateStats 更新统计信息
func (m *RealtimeMonitor) updateStats(bytesSent int, bytesReceived int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.MessagesSent++
	m.stats.LastMessageTime = time.Now()
}

// SetAlertSystem 设置告警系统
func (m *RealtimeMonitor) SetAlertSystem(alertSystem *AlertSystem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertSystem = alertSystem
}

// 消息结构体定义

// MarketDataMessage 市场数据消息
type MarketDataMessage struct {
	Symbol         string    `json:"symbol"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         int64     `json:"volume"`
	Change         float64   `json:"change"`
	ChangePercent  float64   `json:"change_percent"`
	Timestamp      time.Time `json:"timestamp"`
}

// StrategySignalMessage 策略信号消息
type StrategySignalMessage struct {
	Symbol      string    `json:"symbol"`
	SignalType  string    `json:"signal_type"`  // buy, sell, hold
	Strength    float64   `json:"strength"`     // 0-1
	Price       float64   `json:"price"`
	TargetPrice float64   `json:"target_price"`
	StopLoss    float64   `json:"stop_loss"`
	Strategy    string    `json:"strategy"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
}

// TradeEventMessage 交易事件消息
type TradeEventMessage struct {
	Symbol    string    `json:"symbol"`
	Action    string    `json:"action"`    // buy, sell
	Quantity  int64     `json:"quantity"`
	Price     float64   `json:"price"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`   // success, failed
	Strategy  string    `json:"strategy"`
	Timestamp time.Time `json:"timestamp"`
}

// RiskAlertMessage 风险告警消息
type RiskAlertMessage struct {
	Level     string    `json:"level"`     // info, warning, error, critical
	Message   string    `json:"message"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// SystemStatusMessage 系统状态消息
type SystemStatusMessage struct {
	Component  string    `json:"component"`  // scheduler, strategy, risk, etc.
	Status    string    `json:"status"`    // running, stopped, error
	Message   string    `json:"message"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// ClientMessage 客户端消息
type ClientMessage struct {
	Type  string `json:"type"`  // subscribe, unsubscribe, ping
	Topic string `json:"topic"`
}

// 工具函数

func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}