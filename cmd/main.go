package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    "cloudquant/db"
    qhttp "cloudquant/http"
    "cloudquant/llm"
    "cloudquant/ml"
    "gopkg.in/yaml.v2"
)

type Config struct {
    Symbols  []string `yaml:"symbols"`
    Database struct {
        Path string `yaml:"path"`
    } `yaml:"database"`
    Http struct {
        Port int `yaml:"port"`
    } `yaml:"http"`
    Log struct {
        Level string `yaml:"level"`
    } `yaml:"log"`
    LLM struct {
        Provider  string        `yaml:"provider"`
        APIKey    string        `yaml:"api_key"`
        Model     string        `yaml:"model"`
        Timeout   time.Duration `yaml:"timeout"`
        MaxTokens int           `yaml:"max_tokens"`
    } `yaml:"llm"`
    ML struct {
        ModelType     string `yaml:"model_type"`
        ModelPath     string `yaml:"model_path"`
        MaxTreeDepth  int    `yaml:"max_tree_depth"`
        TrainInterval string `yaml:"train_interval"`
        Features      struct {
            LookbackDays  int `yaml:"lookback_days"`
            LookaheadDays int `yaml:"lookahead_days"`
        } `yaml:"features"`
        Training struct {
            MinDataPoints int     `yaml:"min_data_points"`
            TestRatio     float64 `yaml:"test_ratio"`
        } `yaml:"training"`
    } `yaml:"ml"`
}

func main() {
    // Look for config in root even if run from cmd/
    configPath := "config.yaml"
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        configPath = filepath.Join("..", "config.yaml")
    }

    // 1. Load config
    config, err := loadConfig(configPath)
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 2. Initialize database
    // Adjust DB path if needed
    if !filepath.IsAbs(config.Database.Path) && configPath == filepath.Join("..", "config.yaml") {
        config.Database.Path = filepath.Join("..", config.Database.Path)
    }
    
    if err := db.InitDB(config.Database.Path); err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    log.Printf("Database initialized at %s", config.Database.Path)

    initializeServices(config)

    // 3. Start HTTP server
    server := qhttp.NewServer(config.Http.Port)
    go func() {
        if err := server.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTP server failed: %v", err)
        }
    }()

    // 4. Handle graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down...")

    if err := server.Stop(); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    }

    log.Println("Exiting")
}

func loadConfig(path string) (*Config, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var config Config
    if err := yaml.NewDecoder(file).Decode(&config); err != nil {
        return nil, err
    }
    return &config, nil
}

func initializeServices(config *Config) {
    if config == nil {
        return
    }
    analyzer := llm.NewDeepSeekAnalyzer(config.LLM.APIKey, config.LLM.Model, config.LLM.Timeout, config.LLM.MaxTokens)
    qhttp.SetAnalyzer(analyzer)

    if config.ML.ModelType != "" && config.ML.ModelPath != "" {
        if model, err := ml.LoadModel(config.ML.ModelType, config.ML.ModelPath); err == nil {
            qhttp.SetModelProvider(model)
        }
    }

    qhttp.SetTrainingConfig(qhttp.TrainingConfig{
        Symbol:       firstSymbol(config.Symbols),
        Days:         500,
        ModelType:    config.ML.ModelType,
        ModelPath:    config.ML.ModelPath,
        MaxTreeDepth: config.ML.MaxTreeDepth,
        TestRatio:    config.ML.Training.TestRatio,
    })
}

func firstSymbol(symbols []string) string {
    if len(symbols) == 0 {
        return ""
    }
    return symbols[0]
}
