package strategies

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/yaml.v2"
)

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name       string                 `yaml:"name"`       // 策略名称
	Type       StrategyType           `yaml:"type"`       // 策略类型
	Enabled    bool                   `yaml:"enabled"`    // 是否启用
	Weight     float64                `yaml:"weight"`     // 策略权重
	Parameters map[string]interface{} `yaml:"parameters"` // 策略参数
	Priority   int                    `yaml:"priority"`   // 优先级
}

// StrategyLoader 策略加载器
type StrategyLoader struct {
	strategies map[string]Strategy              // 已注册的策略
	factories  map[StrategyType]StrategyFactory // 策略工厂
}

// StrategyFactory 策略工厂接口
type StrategyFactory func() Strategy

// NewStrategyLoader 创建策略加载器
func NewStrategyLoader() *StrategyLoader {
	loader := &StrategyLoader{
		strategies: make(map[string]Strategy),
		factories:  make(map[StrategyType]StrategyFactory),
	}

	// 注册内置策略工厂
	loader.RegisterFactory(MAStrategyType, NewMAStrategy)
	loader.RegisterFactory(RSIStrategyType, NewRSIStrategy)
	loader.RegisterFactory(AIStrategyType, NewAIStrategy)
	loader.RegisterFactory(MLStrategyType, NewMLStrategy)

	return loader
}

// RegisterFactory 注册策略工厂
func (l *StrategyLoader) RegisterFactory(strategyType StrategyType, factory StrategyFactory) {
	l.factories[strategyType] = factory
	log.Printf("Registered strategy factory: %s", strategyType)
}

// LoadStrategies 从配置加载策略
func (l *StrategyLoader) LoadStrategies(configs []StrategyConfig) error {
	for _, config := range configs {
		if !config.Enabled {
			log.Printf("Strategy %s is disabled, skipping", config.Name)
			continue
		}

		strategy, err := l.CreateStrategy(config)
		if err != nil {
			log.Printf("Failed to create strategy %s: %v", config.Name, err)
			continue
		}

		// 初始化策略
		ctx := context.Background()
		if err := strategy.Init(ctx, "", config.Parameters); err != nil {
			log.Printf("Failed to initialize strategy %s: %v", config.Name, err)
			continue
		}

		l.strategies[config.Name] = strategy
		log.Printf("Successfully loaded strategy: %s (type: %s, weight: %.2f)",
			config.Name, config.Type, config.Weight)
	}

	log.Printf("Loaded %d strategies", len(l.strategies))
	return nil
}

// CreateStrategy 根据配置创建策略
func (l *StrategyLoader) CreateStrategy(config StrategyConfig) (Strategy, error) {
	factory, exists := l.factories[config.Type]
	if !exists {
		return nil, fmt.Errorf("strategy type %s not registered", config.Type)
	}

	strategy := factory()
	if base, ok := strategy.(*BaseStrategy); ok {
		base.name = config.Name
		base.weight = config.Weight
		base.enabled = config.Enabled
		if err := base.UpdateParameters(config.Parameters); err != nil {
			return nil, fmt.Errorf("failed to update parameters for %s: %v", config.Name, err)
		}
	}

	return strategy, nil
}

// GetStrategy 获取策略
func (l *StrategyLoader) GetStrategy(name string) (Strategy, bool) {
	strategy, exists := l.strategies[name]
	return strategy, exists
}

// GetAllStrategies 获取所有策略
func (l *StrategyLoader) GetAllStrategies() map[string]Strategy {
	result := make(map[string]Strategy)
	for name, strategy := range l.strategies {
		result[name] = strategy
	}
	return result
}

// GetEnabledStrategies 获取所有启用的策略
func (l *StrategyLoader) GetEnabledStrategies() map[string]Strategy {
	result := make(map[string]Strategy)
	for name, strategy := range l.strategies {
		if strategy.IsEnabled() {
			result[name] = strategy
		}
	}
	return result
}

// UpdateStrategy 更新策略配置
func (l *StrategyLoader) UpdateStrategy(name string, config StrategyConfig) error {
	strategy, exists := l.strategies[name]
	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	// 更新策略权重
	strategy.SetWeight(config.Weight)

	// 更新启用状态
	strategy.SetEnabled(config.Enabled)

	// 更新参数
	if err := strategy.UpdateParameters(config.Parameters); err != nil {
		return fmt.Errorf("failed to update parameters: %v", err)
	}

	log.Printf("Updated strategy %s: enabled=%v, weight=%.2f",
		name, config.Enabled, config.Weight)

	return nil
}

// RemoveStrategy 移除策略
func (l *StrategyLoader) RemoveStrategy(name string) error {
	if _, exists := l.strategies[name]; !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	delete(l.strategies, name)
	log.Printf("Removed strategy: %s", name)
	return nil
}

// ValidateConfig 验证策略配置
func (l *StrategyLoader) ValidateConfig(config *StrategyConfig) error {
	if config.Name == "" {
		return fmt.Errorf("strategy name cannot be empty")
	}

	if config.Type == "" {
		return fmt.Errorf("strategy type cannot be empty")
	}

	if config.Weight < 0 || config.Weight > 1 {
		return fmt.Errorf("strategy weight must be between 0 and 1, got %.2f", config.Weight)
	}

	// 检查策略类型是否已注册
	if _, exists := l.factories[config.Type]; !exists {
		return fmt.Errorf("strategy type %s is not registered", config.Type)
	}

	return nil
}

// LoadFromYAML 从YAML文件加载策略配置
func (l *StrategyLoader) LoadFromYAML(filename string) ([]StrategyConfig, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var configs []StrategyConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	// 验证所有配置
	for i, config := range configs {
		if err := l.ValidateConfig(&config); err != nil {
			return nil, fmt.Errorf("invalid config at index %d: %v", i, err)
		}
	}

	return configs, nil
}

// SaveToYAML 保存策略配置到YAML文件
func (l *StrategyLoader) SaveToYAML(filename string, configs []StrategyConfig) error {
	data, err := yaml.Marshal(configs)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}

	if err := writeFile(filename, data); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	log.Printf("Saved %d strategies to %s", len(configs), filename)
	return nil
}

// GetStrategyNames 获取所有策略名称
func (l *StrategyLoader) GetStrategyNames() []string {
	names := make([]string, 0, len(l.strategies))
	for name := range l.strategies {
		names = append(names, name)
	}
	return names
}

// GetStrategyCount 获取策略数量
func (l *StrategyLoader) GetStrategyCount() int {
	return len(l.strategies)
}

// GetEnabledStrategyCount 获取启用策略数量
func (l *StrategyLoader) GetEnabledStrategyCount() int {
	count := 0
	for _, strategy := range l.strategies {
		if strategy.IsEnabled() {
			count++
		}
	}
	return count
}

// StrategySummary 策略摘要信息
type StrategySummary struct {
	Name        string                 `json:"name"`
	Type        StrategyType           `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Weight      float64                `json:"weight"`
	Parameters  map[string]interface{} `json:"parameters"`
	CreatedAt   string                 `json:"created_at"`
	Description string                 `json:"description"`
}

// GetStrategySummary 获取策略摘要
func (l *StrategyLoader) GetStrategySummary(name string) (*StrategySummary, error) {
	strategy, exists := l.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy %s not found", name)
	}

	summary := &StrategySummary{
		Name:       strategy.GetName(),
		Enabled:    strategy.IsEnabled(),
		Weight:     strategy.GetWeight(),
		Parameters: strategy.GetParameters(),
		CreatedAt:  "unknown", // 可以从元数据中获取
	}

	return summary, nil
}

// GetAllStrategySummaries 获取所有策略摘要
func (l *StrategyLoader) GetAllStrategySummaries() []StrategySummary {
	summaries := make([]StrategySummary, 0, len(l.strategies))

	for _, strategy := range l.strategies {
		summary := StrategySummary{
			Name:       strategy.GetName(),
			Enabled:    strategy.IsEnabled(),
			Weight:     strategy.GetWeight(),
			Parameters: strategy.GetParameters(),
			CreatedAt:  "unknown",
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

// 工具函数 - 文件读写
func readFile(filename string) ([]byte, error) {
	// 这里应该实现真实的文件读取
	// 为了演示，返回模拟数据
	return []byte{}, fmt.Errorf("file reading not implemented")
}

func writeFile(filename string, data []byte) error {
	// 这里应该实现真实的文件写入
	// 为了演示，返回模拟错误
	return fmt.Errorf("file writing not implemented")
}
