// Package http 提供API处理器
package http

import (
	"encoding/json"
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

	// 模拟行业收益率数据
	returns := map[string]float64{
		"银行":   0.05,
		"食品饮料": 0.08,
		"医药生物": 0.03,
		"电力设备": -0.02,
		"电子":   0.12,
		"计算机":  0.06,
	}

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
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1m"
	}

	// 模拟行业收益率历史数据
	returns := map[string][]float64{
		"银行":   {0.01, 0.02, -0.01, 0.015, 0.005},
		"食品饮料": {0.02, 0.03, 0.01, 0.02, 0.015},
		"医药生物": {0.015, 0.01, 0.02, 0.005, 0.01},
		"电力设备": {-0.01, 0.005, -0.02, 0.01, 0.005},
		"电子":   {0.03, 0.025, 0.035, 0.02, 0.03},
	}

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

	// 模拟数据
	portfolioReturns := map[string]float64{
		"sh600000": 0.05,
		"sh601398": 0.03,
		"sh600519": 0.08,
		"sh600036": 0.04,
	}
	benchmarkReturns := map[string]float64{
		"sh600000": 0.04,
		"sh601398": 0.02,
		"sh600519": 0.06,
		"sh600036": 0.03,
	}
	industryMapping := map[string]string{
		"sh600000": "银行",
		"sh601398": "银行",
		"sh600519": "食品饮料",
		"sh600036": "银行",
	}

	attribution := attributionMgr.CalculateAttribution(portfolioReturns, benchmarkReturns, industryMapping)

	respondJSON(w, attribution)
}

func handleRiskMetrics(w http.ResponseWriter, r *http.Request) {
	// 模拟收益率数据
	returns := []float64{0.01, -0.005, 0.02, 0.015, -0.01, 0.008, 0.012, -0.003, 0.005, 0.018}

	calc := risk.NewMetricsCalculator(0.03)
	for _, ret := range returns {
		calc.AddReturn(ret)
	}

	metrics := calc.Calculate()
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

	// 模拟VaR计算
	var result struct {
		Confidence float64   `json:"confidence"`
		Method     string    `json:"method"`
		VaR        float64   `json:"var"`
		CVaR       float64   `json:"cvar"`
		Timestamp  time.Time `json:"timestamp"`
	}
	result.Confidence = confidence
	result.Method = method
	result.VaR = 0.025
	result.CVaR = 0.035
	result.Timestamp = time.Now()

	respondJSON(w, result)
}

func handleRiskFactors(w http.ResponseWriter, r *http.Request) {
	factorModel := risk.NewFactorRiskModel()

	// 模拟持仓数据
	positions := map[string]float64{
		"sh600000": 0.25,
		"sh601398": 0.25,
		"sh600519": 0.25,
		"sh600036": 0.25,
	}

	// 模拟股票因子数据
	stockData := map[string]risk.StockFactorData{
		"sh600000": {Symbol: "sh600000", Beta: 1.1, Size: 0.8, Value: 0.3, Momentum: 0.1},
		"sh601398": {Symbol: "sh601398", Beta: 1.0, Size: 0.9, Value: 0.4, Momentum: 0.05},
		"sh600519": {Symbol: "sh600519", Beta: 0.9, Size: 0.7, Value: 0.2, Momentum: 0.15},
		"sh600036": {Symbol: "sh600036", Beta: 1.05, Size: 0.85, Value: 0.35, Momentum: 0.08},
	}

	exposure := factorModel.CalculateFactorExposure(positions, stockData)
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
	report := &risk.Report{
		Title:       "风险分析报告",
		GeneratedAt: time.Now(),
		Period:      req.Period,
		Summary: risk.ReportSummary{
			InitialCapital: 100000,
			CurrentEquity:  105000,
			TotalReturn:    0.05,
			WinRate:        0.55,
			RiskLevel:      "中等",
			Recommendation: "当前风险状况正常",
		},
	}

	var response []byte
	var contentType string
	var err error

	switch req.Format {
	case "html":
		contentType = "text/html"
		response = []byte(report.ToHTML())
	default:
		contentType = "application/json"
		response, err = report.ToJSON()
		if err != nil {
			http.Error(w, `{"error":"failed to generate report"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(response)
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

// ============ 数据源处理器 ============

func handleProvidersStatus(w http.ResponseWriter, r *http.Request) {
	// 模拟数据源状态
	providers := []map[string]interface{}{
		{
			"name":       "sina",
			"healthy":    true,
			"latency":    150,
			"priority":   1,
			"last_check": time.Now(),
		},
		{
			"name":       "eastmoney",
			"healthy":    true,
			"latency":    200,
			"priority":   2,
			"last_check": time.Now(),
		},
		{
			"name":       "tencent",
			"healthy":    true,
			"latency":    180,
			"priority":   3,
			"last_check": time.Now(),
		},
	}

	respondJSON(w, providers)
}

func handleProvidersHealth(w http.ResponseWriter, r *http.Request) {
	handleProvidersStatus(w, r)
}

func handleMarketAnomalies(w http.ResponseWriter, r *http.Request) {
	// 模拟异常事件
	anomalies := []map[string]interface{}{
		{
			"type":      "price_jump",
			"symbol":    "sh600519",
			"severity":  "medium",
			"message":   "价格跳变超过5%",
			"timestamp": time.Now().Add(-time.Hour),
		},
	}

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
	respondJSON(w, map[string]interface{}{
		"overall_score":  95,
		"latency_score":  90,
		"accuracy_score": 98,
		"coverage_score": 96,
		"timestamp":      time.Now(),
	})
}

// respondJSON 统一JSON响应
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}
