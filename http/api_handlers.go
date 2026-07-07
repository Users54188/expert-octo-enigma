// Package http 提供API处理器
package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"cloudquant/market/industry"
	"cloudquant/monitoring"
	"cloudquant/trading/risk"
)

// RegisterAPIHandlers 注册所有API处理器
func RegisterAPIHandlers(mux *http.ServeMux) {
	// 行业数据API
	mux.HandleFunc("GET /api/industry/exposure", handleIndustryExposure)
	mux.HandleFunc("GET /api/industry/rotation", handleIndustryRotation)
	mux.HandleFunc("GET /api/industry/{symbol}/info", handleIndustryInfo)
	mux.HandleFunc("GET /api/industry/benchmark", handleIndustryBenchmark)
	mux.HandleFunc("GET /api/industry/correlation", handleIndustryCorrelation)
	mux.HandleFunc("GET /api/industry/list", handleIndustryList)
	mux.HandleFunc("GET /api/industry/{industry}/stocks", handleIndustryStocks)

	// 风险模型API
	mux.HandleFunc("GET /api/risk/curve", handleRiskCurve)
	mux.HandleFunc("GET /api/risk/attribution", handleRiskAttribution)
	mux.HandleFunc("GET /api/risk/metrics", handleRiskMetrics)
	mux.HandleFunc("GET /api/risk/var", handleRiskVaR)
	mux.HandleFunc("GET /api/risk/factors", handleRiskFactors)
	mux.HandleFunc("POST /api/risk/report", handleRiskReport)

	// 可视化API
	mux.HandleFunc("GET /api/visualization/equity", handleVisualizationEquity)
	mux.HandleFunc("GET /api/visualization/heatmap", handleVisualizationHeatmap)

	// 回放API
	mux.HandleFunc("POST /api/replay/start", handleReplayStart)
	mux.HandleFunc("POST /api/replay/pause", handleReplayPause)
	mux.HandleFunc("POST /api/replay/resume", handleReplayResume)
	mux.HandleFunc("POST /api/replay/stop", handleReplayStop)
	mux.HandleFunc("GET /api/replay/{id}/status", handleReplayStatus)
	mux.HandleFunc("GET /api/replay/list", handleReplayList)

	// 数据源API
	mux.HandleFunc("GET /api/providers/status", handleProvidersStatus)
	mux.HandleFunc("GET /api/providers/health", handleProvidersHealth)
	mux.HandleFunc("GET /api/market/anomalies", handleMarketAnomalies)
	mux.HandleFunc("POST /api/providers/switch", handleProviderSwitch)
	mux.HandleFunc("GET /api/market/quality", handleMarketQuality)
}

// ============ 行业数据处理器 ============

func handleIndustryExposure(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	benchmark := r.URL.Query().Get("benchmark")
	if benchmark == "" {
		benchmark = "csi300"
	}

	// 获取缓存
	cache, err := industry.GetGlobalCache("./data/industries.json")
	if err != nil {
		http.Error(w, `{"error":"failed to load industry data"}`, http.StatusInternalServerError)
		return
	}

	// 构建模拟持仓（实际应从持仓管理器获取）
	positions := map[string]float64{
		"sh600000": 0.25,
		"sh601398": 0.25,
		"sh600519": 0.25,
		"sh600036": 0.25,
	}

	analyzer := industry.NewAnalyzer(cache)
	exposure := analyzer.CalculateExposure(positions, benchmark)

	respondJSON(w, exposure)
}

func handleIndustryRotation(w http.ResponseWriter, r *http.Request) {
	lookbackDays := 30
	if days := r.URL.Query().Get("days"); days != "" {
		if d, err := strconv.Atoi(days); err == nil {
			lookbackDays = d
		}
	}

	threshold := 0.02
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = v
		}
	}

	// TODO: 替换为真实行业收益率数据
	returns := mockProvider.GetIndustryRotationData()

	cache, _ := industry.GetGlobalCache("./data/industries.json")
	analyzer := industry.NewAnalyzer(cache)
	rotations := analyzer.DetectSectorRotation(returns, lookbackDays, threshold)

	respondJSON(w, rotations)
}

func handleIndustryInfo(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, `{"error":"symbol is required"}`, http.StatusBadRequest)
		return
	}

	cache, err := industry.GetGlobalCache("./data/industries.json")
	if err != nil {
		http.Error(w, `{"error":"failed to load industry data"}`, http.StatusInternalServerError)
		return
	}

	info, exists := cache.GetStockIndustry(symbol)
	if !exists {
		http.Error(w, `{"error":"symbol not found"}`, http.StatusNotFound)
		return
	}

	respondJSON(w, info)
}

func handleIndustryBenchmark(w http.ResponseWriter, r *http.Request) {
	benchmark := r.URL.Query().Get("benchmark")
	if benchmark == "" {
		benchmark = "csi300"
	}

	cache, err := industry.GetGlobalCache("./data/industries.json")
	if err != nil {
		http.Error(w, `{"error":"failed to load industry data"}`, http.StatusInternalServerError)
		return
	}

	weights := cache.GetBenchmarkWeights(benchmark)
	if weights == nil {
		http.Error(w, `{"error":"benchmark not found"}`, http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]interface{}{
		"benchmark": benchmark,
		"weights":   weights,
		"timestamp": time.Now(),
	})
}

func handleIndustryCorrelation(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实行业收益率历史数据
	returns := mockProvider.GetCorrelationReturns()

	cache, _ := industry.GetGlobalCache("./data/industries.json")
	analyzer := industry.NewAnalyzer(cache)
	correlation := analyzer.CalculateCorrelationMatrix(returns, nil)

	respondJSON(w, correlation)
}

func handleIndustryList(w http.ResponseWriter, r *http.Request) {
	cache, err := industry.GetGlobalCache("./data/industries.json")
	if err != nil {
		http.Error(w, `{"error":"failed to load industry data"}`, http.StatusInternalServerError)
		return
	}

	industries := cache.GetIndustryList()
	respondJSON(w, map[string]interface{}{
		"industries": industries,
		"count":      len(industries),
	})
}

func handleIndustryStocks(w http.ResponseWriter, r *http.Request) {
	industryName := r.PathValue("industry")
	if industryName == "" {
		http.Error(w, `{"error":"industry is required"}`, http.StatusBadRequest)
		return
	}

	cache, err := industry.GetGlobalCache("./data/industries.json")
	if err != nil {
		http.Error(w, `{"error":"failed to load industry data"}`, http.StatusInternalServerError)
		return
	}

	stocks := cache.GetStocksByIndustry(industryName)
	respondJSON(w, map[string]interface{}{
		"industry": industryName,
		"stocks":   stocks,
		"count":    len(stocks),
	})
}

// ============ 风险模型处理器 ============

func handleRiskCurve(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil {
			days = v
		}
	}

	// 模拟资金曲线数据
	curve := make([]map[string]interface{}, days)
	equity := 100000.0
	baseDate := time.Now().AddDate(0, 0, -days)

	for i := 0; i < days; i++ {
		change := (float64(i%10) - 5) / 1000
		equity = equity * (1 + change)

		curve[i] = map[string]interface{}{
			"date":         baseDate.AddDate(0, 0, i).Format("2006-01-02"),
			"equity":       equity,
			"daily_return": change,
			"drawdown":     float64(i%20) / 1000,
		}
	}

	respondJSON(w, map[string]interface{}{
		"curve": curve,
		"days":  days,
	})
}

func handleRiskAttribution(w http.ResponseWriter, r *http.Request) {
	// 创建归因管理器
	attributionMgr := risk.NewAttributionManager()

	// TODO: 替换为真实投资组合数据
	portfolioReturns := mockProvider.GetPortfolioReturns()
	benchmarkReturns := mockProvider.GetBenchmarkReturns()
	industryMapping := mockProvider.GetIndustryMapping()

	attribution := attributionMgr.CalculateAttribution(portfolioReturns, benchmarkReturns, industryMapping)

	respondJSON(w, attribution)
}

func handleRiskMetrics(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实风险指标计算
	metrics := mockProvider.GetRiskMetrics()
	respondJSON(w, metrics)
}

func handleRiskVaR(w http.ResponseWriter, r *http.Request) {
	confidence := 0.95
	if c := r.URL.Query().Get("confidence"); c != "" {
		if v, err := strconv.ParseFloat(c, 64); err == nil {
			confidence = v
		}
	}

	method := r.URL.Query().Get("method")
	if method == "" {
		method = "historical"
	}

	// TODO: 替换为真实VaR计算
	result := mockProvider.GetVaRData(confidence, method)
	respondJSON(w, result)
}

func handleRiskFactors(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实因子暴露计算
	exposure := mockProvider.GetFactorExposure()
	respondJSON(w, exposure)
}

func handleRiskReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Period string `json:"period"`
		Format string `json:"format"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Period == "" {
		req.Period = "1m"
	}
	if req.Format == "" {
		req.Format = "json"
	}

	// 生成模拟报告
	report := map[string]interface{}{
		"title":        "风险分析报告",
		"generated_at": time.Now(),
		"period":       req.Period,
		"summary": map[string]interface{}{
			"initial_capital": 100000,
			"current_equity":  105000,
			"total_return":    0.05,
			"win_rate":        0.55,
			"risk_level":      "中等",
			"recommendation":  "当前风险状况正常",
		},
	}

	var response []byte
	var contentType string
	var err error

	switch req.Format {
	case "html":
		contentType = "text/html; charset=utf-8"
		response = []byte("<html><body><h1>风险分析报告</h1><p>当前风险状况正常</p></body></html>")
	default:
		contentType = "application/json"
		response, err = json.Marshal(report)
		if err != nil {
			http.Error(w, `{"error":"failed to generate report"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(response) // Best-effort response writing
}

// ============ 可视化处理器 ============

func handleVisualizationEquity(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil {
			days = v
		}
	}

	// 模拟权益曲线
	equityCurve := make([]map[string]interface{}, days)
	equity := 100000.0
	baseDate := time.Now().AddDate(0, 0, -days)

	for i := 0; i < days; i++ {
		change := (float64(i%10) - 5) / 1000
		equity = equity * (1 + change)

		equityCurve[i] = map[string]interface{}{
			"date":   baseDate.AddDate(0, 0, i).Format("2006-01-02"),
			"equity": equity,
			"return": change,
		}
	}

	respondJSON(w, map[string]interface{}{
		"equity_curve": equityCurve,
		"days":         days,
	})
}

func handleVisualizationHeatmap(w http.ResponseWriter, r *http.Request) {
	symbols := []string{"sh600000", "sh601398", "sh600519", "sh600036"}
	periods := []string{"1D", "1W", "1M", "3M", "YTD"}

	// 模拟热力图数据
	data := make([][]float64, len(symbols))
	for i := range data {
		data[i] = make([]float64, len(periods))
		for j := range data[i] {
			data[i][j] = (float64((i+j)%20) - 10) / 100
		}
	}

	respondJSON(w, map[string]interface{}{
		"symbols": symbols,
		"periods": periods,
		"data":    data,
	})
}

// ============ 回放处理器 ============

var replayEngine *monitoring.ReplayEngine

// SetReplayEngine 设置回放引擎
func SetReplayEngine(engine *monitoring.ReplayEngine) {
	replayEngine = engine
}

func handleReplayStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Symbol    string    `json:"symbol"`
		StartDate time.Time `json:"start_date"`
		EndDate   time.Time `json:"end_date"`
		Speed     float64   `json:"speed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Speed == 0 {
		req.Speed = 1
	}

	if replayEngine == nil {
		// 创建Mock回放引擎
		replayEngine = monitoring.NewReplayEngine(monitoring.NewMockReplayDataProvider())
	}

	session, err := replayEngine.StartSession(req.Symbol, req.StartDate, req.EndDate, req.Speed)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	respondJSON(w, session)
}

func handleReplayPause(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if replayEngine == nil {
		http.Error(w, `{"error":"replay engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	if err := replayEngine.PauseSession(req.ID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]string{"status": "paused"})
}

func handleReplayResume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if replayEngine == nil {
		http.Error(w, `{"error":"replay engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	if err := replayEngine.ResumeSession(req.ID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]string{"status": "resumed"})
}

func handleReplayStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if replayEngine == nil {
		http.Error(w, `{"error":"replay engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	if err := replayEngine.StopSession(req.ID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]string{"status": "stopped"})
}

func handleReplayStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}

	if replayEngine == nil {
		http.Error(w, `{"error":"replay engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	session, err := replayEngine.GetSession(id)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	respondJSON(w, session)
}

func handleReplayList(w http.ResponseWriter, r *http.Request) {
	if replayEngine == nil {
		respondJSON(w, []interface{}{})
		return
	}

	sessions := replayEngine.GetAllSessions()
	respondJSON(w, sessions)
}

// mockProvider 模拟数据提供者（全局实例）
var mockProvider = NewMockDataProvider()

func handleProvidersStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实数据源健康检查
	providers := mockProvider.GetProviderStatus()
	respondJSON(w, providers)
}

func handleProvidersHealth(w http.ResponseWriter, r *http.Request) {
	handleProvidersStatus(w, r)
}

func handleMarketAnomalies(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实异常检测
	anomalies := mockProvider.GetMarketAnomalies()
	respondJSON(w, anomalies)
}

func handleProviderSwitch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]interface{}{
		"status":   "success",
		"provider": req.Provider,
		"message":  "切换到 " + req.Provider,
	})
}

func handleMarketQuality(w http.ResponseWriter, r *http.Request) {
	// TODO: 替换为真实数据质量评估
	quality := mockProvider.GetMarketQuality()
	respondJSON(w, quality)
}

// respondJSON 统一JSON响应
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}
