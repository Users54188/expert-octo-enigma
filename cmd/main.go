package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"path/filepath"

	"cloudquant/db"
	qhttp "cloudquant/http"
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
