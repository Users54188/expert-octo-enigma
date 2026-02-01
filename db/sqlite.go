package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"cloudquant/market"
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
	);`

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
		if ma5.Valid { k.Indicators.MA5 = ma5.Float64 }
		if ma20.Valid { k.Indicators.MA20 = ma20.Float64 }
		if rsi.Valid { k.Indicators.RSI = rsi.Float64 }
		if macd.Valid { k.Indicators.MACD = macd.Float64 }
		k.Indicators.Timestamp = k.Timestamp
		klines = append(klines, k)
	}

	// Reverse to get chronological order if needed, but Query requested limit usually means latest first.
	// The caller can decide.
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
