package db

import (
	"database/sql"
	"errors"
	"time"

	"cloudquant/market"
	"cloudquant/ml"
	_ "github.com/mattn/go-sqlite3"
)

var database *sql.DB

// InitDB initializes the SQLite database
func InitDB(path string) error {
	var err error
	database, err = sql.Open("sqlite3", path)
	if err != nil {
		return err
	}

	query := `
    CREATE TABLE IF NOT EXISTS klines (
        id INTEGER PRIMARY KEY,
        symbol VARCHAR(20),
        open REAL,
        high REAL,
        low REAL,
        close REAL,
        volume INTEGER,
        timestamp DATETIME,
        UNIQUE(symbol, timestamp)
    );
    CREATE TABLE IF NOT EXISTS indicators (
        id INTEGER PRIMARY KEY,
        symbol VARCHAR(20),
        ma5 REAL,
        ma20 REAL,
        rsi REAL,
        macd REAL,
        timestamp DATETIME,
        UNIQUE(symbol, timestamp)
    );
    CREATE TABLE IF NOT EXISTS features (
        id INTEGER PRIMARY KEY,
        symbol VARCHAR(20),
        ma5 REAL,
        ma20 REAL,
        ma60 REAL,
        rsi REAL,
        macd REAL,
        macd_signal REAL,
        price_change REAL,
        volume_change REAL,
        volatility REAL,
        timestamp DATETIME,
        UNIQUE(symbol, timestamp)
    );
    CREATE TABLE IF NOT EXISTS predictions (
        id INTEGER PRIMARY KEY,
        symbol VARCHAR(20),
        predicted_label INTEGER,
        confidence REAL,
        ai_trend VARCHAR(20),
        ai_action VARCHAR(20),
        timestamp DATETIME,
        UNIQUE(symbol, timestamp)
    );
    CREATE TABLE IF NOT EXISTS training_log (
        id INTEGER PRIMARY KEY,
        model_name VARCHAR(50),
        accuracy REAL,
        precision REAL,
        recall REAL,
        trained_at DATETIME,
        data_points INTEGER
    );
    CREATE TABLE IF NOT EXISTS trades (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        trade_id TEXT NOT NULL,
        order_id TEXT NOT NULL,
        symbol TEXT NOT NULL,
        type TEXT NOT NULL,
        price REAL NOT NULL,
        amount INTEGER NOT NULL,
        commission REAL DEFAULT 0,
        trade_time DATETIME NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(trade_id)
    );
    CREATE TABLE IF NOT EXISTS orders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        order_id TEXT NOT NULL,
        symbol TEXT NOT NULL,
        type TEXT NOT NULL,
        price REAL NOT NULL,
        amount INTEGER NOT NULL,
        filled_amount INTEGER DEFAULT 0,
        status TEXT NOT NULL,
        order_time DATETIME NOT NULL,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(order_id)
    );
    CREATE TABLE IF NOT EXISTS positions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        symbol TEXT NOT NULL UNIQUE,
        amount INTEGER DEFAULT 0,
        cost_price REAL DEFAULT 0,
        total_cost REAL DEFAULT 0,
        current_price REAL DEFAULT 0,
        market_value REAL DEFAULT 0,
        unrealized_pnl REAL DEFAULT 0,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS daily_performance (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        date TEXT NOT NULL UNIQUE,
        open_equity REAL DEFAULT 0,
        close_equity REAL DEFAULT 0,
        daily_pnl REAL DEFAULT 0,
        daily_pnl_percent REAL DEFAULT 0,
        trade_count INTEGER DEFAULT 0,
        buy_count INTEGER DEFAULT 0,
        sell_count INTEGER DEFAULT 0,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `

	_, err = database.Exec(query)
	return err
}

// SaveKLine saves K-line and its indicators to the database
func SaveKLine(kline market.KLine) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
        INSERT OR REPLACE INTO klines (symbol, open, high, low, close, volume, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		kline.Symbol, kline.Open, kline.High, kline.Low, kline.Close, kline.Volume, kline.Timestamp)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`
        INSERT OR REPLACE INTO indicators (symbol, ma5, ma20, rsi, macd, timestamp)
        VALUES (?, ?, ?, ?, ?, ?)`,
		kline.Symbol, kline.Indicators.MA5, kline.Indicators.MA20, kline.Indicators.RSI, kline.Indicators.MACD, kline.Timestamp)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// QueryKLines queries K-line data for a symbol
func QueryKLines(symbol string, limit int) ([]market.KLine, error) {
	rows, err := database.Query(`
        SELECT k.symbol, k.open, k.high, k.low, k.close, k.volume, k.timestamp,
               i.ma5, i.ma20, i.rsi, i.macd
        FROM klines k
        LEFT JOIN indicators i ON k.symbol = i.symbol AND k.timestamp = i.timestamp
        WHERE k.symbol = ?
        ORDER BY k.timestamp DESC
        LIMIT ?`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var klines []market.KLine
	for rows.Next() {
		var k market.KLine
		var ma5, ma20, rsi, macd sql.NullFloat64
		err := rows.Scan(&k.Symbol, &k.Open, &k.High, &k.Low, &k.Close, &k.Volume, &k.Timestamp,
			&ma5, &ma20, &rsi, &macd)
		if err != nil {
			return nil, err
		}
		if ma5.Valid {
			k.Indicators.MA5 = ma5.Float64
		}
		if ma20.Valid {
			k.Indicators.MA20 = ma20.Float64
		}
		if rsi.Valid {
			k.Indicators.RSI = rsi.Float64
		}
		if macd.Valid {
			k.Indicators.MACD = macd.Float64
		}
		k.Indicators.Timestamp = k.Timestamp
		klines = append(klines, k)
	}

	return klines, nil
}

// GetLatestIndicators gets the latest indicators for a symbol
func GetLatestIndicators(symbol string) (*market.Indicator, error) {
	var i market.Indicator
	err := database.QueryRow(`
        SELECT ma5, ma20, rsi, macd, timestamp
        FROM indicators
        WHERE symbol = ?
        ORDER BY timestamp DESC
        LIMIT 1`, symbol).Scan(&i.MA5, &i.MA20, &i.RSI, &i.MACD, &i.Timestamp)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func SaveFeatures(features ml.MLFeatures) error {
	if database == nil {
		return errors.New("database not initialized")
	}
	_, err := database.Exec(`
        INSERT OR REPLACE INTO features (
            symbol, ma5, ma20, ma60, rsi, macd, macd_signal,
            price_change, volume_change, volatility, timestamp
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
		features.Symbol,
		features.MA5,
		features.MA20,
		features.MA60,
		features.RSI,
		features.MACD,
		features.MACDSignal,
		features.PriceChange,
		features.VolumeChange,
		features.Volatility,
		features.Timestamp,
	)
	return err
}

func SavePredictions(predictions []int, confidences []float64, symbol string) error {
	if database == nil {
		return errors.New("database not initialized")
	}
	if len(predictions) != len(confidences) {
		return errors.New("predictions/confidences length mismatch")
	}
	if symbol == "" {
		return errors.New("symbol required")
	}
	if len(predictions) == 0 {
		return nil
	}

	stmt, err := database.Prepare(`
        INSERT OR REPLACE INTO predictions (
            symbol, predicted_label, confidence, ai_trend, ai_action, timestamp
        ) VALUES (?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for i, label := range predictions {
		if _, err := stmt.Exec(symbol, label, confidences[i], "", "", now); err != nil {
			return err
		}
	}
	return nil
}

type TrainingLog struct {
	ModelName  string    `json:"model_name"`
	Accuracy   float64   `json:"accuracy"`
	Precision  float64   `json:"precision"`
	Recall     float64   `json:"recall"`
	TrainedAt  time.Time `json:"trained_at"`
	DataPoints int       `json:"data_points"`
}

func LoadTrainingLog() ([]TrainingLog, error) {
	if database == nil {
		return nil, errors.New("database not initialized")
	}
	rows, err := database.Query(`
        SELECT model_name, accuracy, precision, recall, trained_at, data_points
        FROM training_log
        ORDER BY trained_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]TrainingLog, 0)
	for rows.Next() {
		var log TrainingLog
		if err := rows.Scan(&log.ModelName, &log.Accuracy, &log.Precision, &log.Recall, &log.TrainedAt, &log.DataPoints); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}
