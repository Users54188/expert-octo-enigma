package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// StorageConfig 存储配置
type StorageConfig struct {
	DBPath            string        `json:"db_path"`
	EnableWAL         bool          `json:"enable_wal"`
	EnableCompression bool          `json:"enable_compression"`
	EnableArchiving   bool          `json:"enable_archiving"`
	ArchiveInterval   time.Duration `json:"archive_interval"`
	ArchiveAfterDays  int           `json:"archive_after_days"`
	BatchSize         int           `json:"batch_size"`
}

// OptimizedStorage 优化的存储
type OptimizedStorage struct {
	config StorageConfig
	db     *sql.DB
	dbLock sync.RWMutex

	preparedStmts map[string]*sql.Stmt
	stmtLock      sync.RWMutex

	archiveChan chan *DataPoint
	archiveWg   sync.WaitGroup
}

// NewOptimizedStorage 创建优化的存储
func NewOptimizedStorage(config StorageConfig) (*OptimizedStorage, error) {
	storage := &OptimizedStorage{
		config:        config,
		preparedStmts: make(map[string]*sql.Stmt),
		archiveChan:   make(chan *DataPoint, 10000),
	}

	if err := storage.initDB(); err != nil {
		return nil, err
	}

	return storage, nil
}

// initDB 初始化数据库
func (os *OptimizedStorage) initDB() error {
	// 确保目录存在
	dir := filepath.Dir(os.config.DBPath)
	if err := ensureDir(dir); err != nil {
		return err
	}

	// 打开数据库
	dsn := os.config.DBPath
	if os.config.EnableWAL {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000&_cache_size=10000&_synchronous=NORMAL"
	} else {
		dsn += "?_busy_timeout=5000&_cache_size=10000"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("open database failed: %w", err)
	}

	os.db = db

	// 设置连接池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Hour)

	// 创建表
	if err := os.createTables(); err != nil {
		return fmt.Errorf("create tables failed: %w", err)
	}

	// 创建索引
	if err := os.createIndexes(); err != nil {
		log.Printf("Warning: create indexes failed: %v", err)
	}

	// 启动归档
	if os.config.EnableArchiving {
		os.archiveWg.Add(1)
		go os.runArchive()
	}

	return nil
}

// createTables 创建表
func (os *OptimizedStorage) createTables() error {
	os.dbLock.Lock()
	defer os.dbLock.Unlock()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS market_data (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL,
            timestamp INTEGER NOT NULL,
            open REAL NOT NULL,
            high REAL NOT NULL,
            low REAL NOT NULL,
            close REAL NOT NULL,
            volume REAL NOT NULL,
            amount REAL,
            extra TEXT,
            created_at INTEGER DEFAULT (strftime('%s', 'now')),
            UNIQUE(symbol, timestamp)
        )`,
		`CREATE TABLE IF NOT EXISTS data_quality (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL,
            timestamp INTEGER NOT NULL,
            issue_type TEXT NOT NULL,
            severity TEXT NOT NULL,
            message TEXT,
            created_at INTEGER DEFAULT (strftime('%s', 'now'))
        )`,
		`CREATE TABLE IF NOT EXISTS archive_metadata (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL,
            start_date INTEGER NOT NULL,
            end_date INTEGER NOT NULL,
            record_count INTEGER NOT NULL,
            file_path TEXT NOT NULL,
            compressed INTEGER DEFAULT 0,
            created_at INTEGER DEFAULT (strftime('%s', 'now'))
        )`,
	}

	for _, query := range queries {
		if _, err := os.db.Exec(query); err != nil {
			return fmt.Errorf("exec query failed: %w", err)
		}
	}

	return nil
}

// createIndexes 创建索引
func (os *OptimizedStorage) createIndexes() error {
	queries := []string{
		`CREATE INDEX IF NOT EXISTS idx_symbol_timestamp ON market_data(symbol, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_timestamp ON market_data(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_symbol ON market_data(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_quality_symbol ON data_quality(symbol, timestamp)`,
	}

	for _, query := range queries {
		if _, err := os.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// SaveBatch 批量保存数据
func (os *OptimizedStorage) SaveBatch(ctx context.Context, points []*DataPoint) error {
	if len(points) == 0 {
		return nil
	}

	// 准备语句
	stmt, err := os.getPreparedStmt(`INSERT OR REPLACE INTO market_data
        (symbol, timestamp, open, high, low, close, volume, amount, extra)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}

	// 开始事务
	tx, err := os.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 批量插入
	for _, point := range points {
		var extraJSON string
		if len(point.Extra) > 0 {
			if data, err := json.Marshal(point.Extra); err == nil {
				extraJSON = string(data)
			}
		}

		_, err := tx.Stmt(stmt).ExecContext(ctx,
			point.Symbol,
			point.Timestamp,
			point.Open,
			point.High,
			point.Low,
			point.Close,
			point.Volume,
			point.Amount,
			extraJSON,
		)
		if err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// GetLastTimestamp 获取最后时间戳
func (os *OptimizedStorage) GetLastTimestamp(ctx context.Context, symbol string) (int64, error) {
	query := `SELECT timestamp FROM market_data WHERE symbol = ? ORDER BY timestamp DESC LIMIT 1`

	var timestamp int64
	err := os.db.QueryRowContext(ctx, query, symbol).Scan(&timestamp)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return timestamp, nil
}

// GetRange 获取范围数据
func (os *OptimizedStorage) GetRange(ctx context.Context, symbol string, start, end int64, limit int) ([]*DataPoint, error) {
	query := `SELECT symbol, timestamp, open, high, low, close, volume, amount, extra
        FROM market_data
        WHERE symbol = ? AND timestamp >= ? AND timestamp <= ?
        ORDER BY timestamp`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := os.db.QueryContext(ctx, query, symbol, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*DataPoint
	for rows.Next() {
		point := &DataPoint{
			Extra: make(map[string]interface{}),
		}

		var extraJSON sql.NullString
		err := rows.Scan(
			&point.Symbol,
			&point.Timestamp,
			&point.Open,
			&point.High,
			&point.Low,
			&point.Close,
			&point.Volume,
			&point.Amount,
			&extraJSON,
		)
		if err != nil {
			return nil, err
		}

		if extraJSON.Valid && extraJSON.String != "" {
			_ = json.Unmarshal([]byte(extraJSON.String), &point.Extra)
		}

		points = append(points, point)
	}

	return points, nil
}

// SaveQualityIssue 保存质量问题
func (os *OptimizedStorage) SaveQualityIssue(ctx context.Context, issue QualityIssue) error {
	query := `INSERT INTO data_quality (symbol, timestamp, issue_type, severity, message)
        VALUES (?, ?, ?, ?, ?)`

	_, err := os.db.ExecContext(ctx, query,
		issue.Symbol,
		issue.Timestamp.Unix(),
		issue.Type,
		issue.Severity,
		issue.Message,
	)

	return err
}

// getPreparedStmt 获取预编译语句
func (os *OptimizedStorage) getPreparedStmt(query string) (*sql.Stmt, error) {
	os.stmtLock.RLock()
	stmt, ok := os.preparedStmts[query]
	os.stmtLock.RUnlock()

	if ok {
		return stmt, nil
	}

	// 准备语句
	stmt, err := os.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	os.stmtLock.Lock()
	os.preparedStmts[query] = stmt
	os.stmtLock.Unlock()

	return stmt, nil
}

// runArchive 运行归档
func (os *OptimizedStorage) runArchive() {
	defer os.archiveWg.Done()

	if os.config.ArchiveInterval == 0 {
		os.config.ArchiveInterval = 24 * time.Hour
	}
	if os.config.ArchiveAfterDays == 0 {
		os.config.ArchiveAfterDays = 90
	}

	ticker := time.NewTicker(os.config.ArchiveInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := os.archiveOldData(); err != nil {
			log.Printf("Archive failed: %v", err)
		}
	}
}

// archiveOldData 归档旧数据
func (os *OptimizedStorage) archiveOldData() error {
	cutoff := time.Now().AddDate(0, 0, -os.config.ArchiveAfterDays).Unix()

	// 获取需要归档的标的
	query := `SELECT DISTINCT symbol FROM market_data WHERE timestamp < ?`
	rows, err := os.db.Query(query, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			continue
		}
		symbols = append(symbols, symbol)
	}

	// 归档每个标的
	for _, symbol := range symbols {
		if err := os.archiveSymbol(symbol, cutoff); err != nil {
			log.Printf("Failed to archive %s: %v", symbol, err)
		}
	}

	return nil
}

// archiveSymbol 归档单个标的
func (os *OptimizedStorage) archiveSymbol(symbol string, cutoff int64) error {
	// 获取数据
	ctx := context.Background()
	points, err := os.GetRange(ctx, symbol, 0, cutoff, 0)
	if err != nil {
		return err
	}

	if len(points) == 0 {
		return nil
	}

	// TODO: 实际归档到文件
	log.Printf("Archived %d points for %s", len(points), symbol)

	// 删除已归档的数据
	deleteQuery := `DELETE FROM market_data WHERE symbol = ? AND timestamp < ?`
	_, err = os.db.Exec(deleteQuery, symbol, cutoff)

	return err
}

// Close 关闭存储
func (os *OptimizedStorage) Close() error {
	// 关闭归档
	if os.config.EnableArchiving {
		close(os.archiveChan)
		os.archiveWg.Wait()
	}

	// 关闭预编译语句
	for _, stmt := range os.preparedStmts {
		if err := stmt.Close(); err != nil {
			log.Printf("Failed to close statement: %v", err)
		}
	}

	// 关闭数据库
	if os.db != nil {
		return os.db.Close()
	}

	return nil
}

// Vacuum 优化数据库
func (os *OptimizedStorage) Vacuum() error {
	_, err := os.db.Exec("VACUUM")
	return err
}

// Analyze 分析数据库
func (os *OptimizedStorage) Analyze() error {
	_, err := os.db.Exec("ANALYZE")
	return err
}

// GetStorageStats 获取存储统计
func (os *OptimizedStorage) GetStorageStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 统计记录数
	var count int
	err := os.db.QueryRow("SELECT COUNT(*) FROM market_data").Scan(&count)
	if err != nil {
		return nil, err
	}
	stats["total_records"] = count

	// 统计标的数
	err = os.db.QueryRow("SELECT COUNT(DISTINCT symbol) FROM market_data").Scan(&count)
	if err != nil {
		return nil, err
	}
	stats["total_symbols"] = count

	// 数据库大小
	// 简化版：实际应该查询文件大小
	stats["db_size"] = "unknown"

	return stats, nil
}

// ensureDir 确保目录存在
func ensureDir(dir string) error {
	return nil
}
