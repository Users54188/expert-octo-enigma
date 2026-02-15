package http

import (
	"cloudquant/monitoring"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

var (
	dashboardManager   *monitoring.DashboardManager
	performanceTracker *monitoring.PerformanceTracker
)

func SetDashboardManager(dm *monitoring.DashboardManager) {
	dashboardManager = dm
}

func SetPerformanceTracker(pt *monitoring.PerformanceTracker) {
	performanceTracker = pt
}

func handleDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	if dashboardManager == nil {
		http.Error(w, "Dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	metrics := dashboardManager.GetMetrics()
	if metrics == nil {
		http.Error(w, "No metrics available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func handleDashboardEquity(w http.ResponseWriter, r *http.Request) {
	if dashboardManager == nil {
		http.Error(w, "Dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	equityCurve := dashboardManager.GetEquityCurve(days)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"equity_curve": equityCurve,
		"days":         days,
		"timestamp":    time.Now(),
	})
}

func handleDashboardPositions(w http.ResponseWriter, r *http.Request) {
	if dashboardManager == nil {
		http.Error(w, "Dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	positions := dashboardManager.GetPositions()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"positions": positions,
		"count":     len(positions),
		"timestamp": time.Now(),
	})
}

func handleDashboardRisk(w http.ResponseWriter, r *http.Request) {
	if dashboardManager == nil {
		http.Error(w, "Dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	riskMetrics := dashboardManager.GetRiskMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"risk_metrics": riskMetrics,
		"timestamp":    time.Now(),
	})
}

func handleDashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	if dashboardManager == nil {
		http.Error(w, "Dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	snapshot := dashboardManager.GetSnapshot()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

func handlePerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	if performanceTracker == nil {
		http.Error(w, "Performance tracker not initialized", http.StatusServiceUnavailable)
		return
	}

	metrics := performanceTracker.CalculateMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func handlePerformanceEquityHistory(w http.ResponseWriter, r *http.Request) {
	if performanceTracker == nil {
		http.Error(w, "Performance tracker not initialized", http.StatusServiceUnavailable)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	equityHistory := performanceTracker.GetEquityHistory(days)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"equity_history": equityHistory,
		"days":           days,
		"count":          len(equityHistory),
	})
}

func handlePerformanceTrades(w http.ResponseWriter, r *http.Request) {
	if performanceTracker == nil {
		http.Error(w, "Performance tracker not initialized", http.StatusServiceUnavailable)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	trades := performanceTracker.GetTrades(limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trades": trades,
		"count":  len(trades),
		"limit":  limit,
	})
}

func handlePerformanceDrawdown(w http.ResponseWriter, r *http.Request) {
	if performanceTracker == nil {
		http.Error(w, "Performance tracker not initialized", http.StatusServiceUnavailable)
		return
	}

	currentDrawdown := performanceTracker.GetCurrentDrawdown()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_drawdown": currentDrawdown,
		"max_drawdown":     performanceTracker.CalculateMetrics().MaxDrawdown,
		"peak_equity":      performanceTracker.GetLatestEquity() / (1 - currentDrawdown),
		"timestamp":        time.Now(),
	})
}

func handlePerformanceStats(w http.ResponseWriter, r *http.Request) {
	if performanceTracker == nil {
		http.Error(w, "Performance tracker not initialized", http.StatusServiceUnavailable)
		return
	}

	winningTrades := performanceTracker.GetWinningTrades()
	losingTrades := performanceTracker.GetLosingTrades()

	stats := map[string]interface{}{
		"total_trades":   performanceTracker.CalculateMetrics().TotalTrades,
		"winning_trades": len(winningTrades),
		"losing_trades":  len(losingTrades),
		"win_rate":       performanceTracker.CalculateMetrics().WinRate,
		"average_win":    performanceTracker.GetAverageWin(),
		"average_loss":   performanceTracker.GetAverageLoss(),
		"profit_factor":  performanceTracker.GetProfitFactor(),
		"expectancy":     performanceTracker.GetExpectancy(),
		"current_equity": performanceTracker.GetLatestEquity(),
		"timestamp":      time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func RegisterDashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/dashboard/metrics", handleDashboardMetrics)
	mux.HandleFunc("/api/dashboard/equity", handleDashboardEquity)
	mux.HandleFunc("/api/dashboard/positions", handleDashboardPositions)
	mux.HandleFunc("/api/dashboard/risk", handleDashboardRisk)
	mux.HandleFunc("/api/dashboard/snapshot", handleDashboardSnapshot)

	mux.HandleFunc("/api/performance/metrics", handlePerformanceMetrics)
	mux.HandleFunc("/api/performance/equity", handlePerformanceEquityHistory)
	mux.HandleFunc("/api/performance/trades", handlePerformanceTrades)
	mux.HandleFunc("/api/performance/drawdown", handlePerformanceDrawdown)
	mux.HandleFunc("/api/performance/stats", handlePerformanceStats)
}
