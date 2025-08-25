package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// MarketHandler 市场分析处理器
type MarketHandler struct {
	binanceService   *service.BinanceService
	technicalService *service.TechnicalAnalysisService
	llmService       *service.LLMAnalysisService
	newsService      *service.NewsService
	logger           *zap.Logger
}

// NewMarketHandler 创建市场分析处理器
func NewMarketHandler(
	binanceService *service.BinanceService,
	technicalService *service.TechnicalAnalysisService,
	llmService *service.LLMAnalysisService,
	newsService *service.NewsService,
	logger *zap.Logger,
) *MarketHandler {
	return &MarketHandler{
		binanceService:   binanceService,
		technicalService: technicalService,
		llmService:       llmService,
		newsService:      newsService,
		logger:           logger,
	}
}

// MarketAnalysisRequest 市场分析请求
type MarketAnalysisRequest struct {
	Symbol   string `json:"symbol" validate:"required"`
	Interval string `json:"interval" validate:"required"`
	Limit    int    `json:"limit"`
}

// LLMPromptRequest LLM提示词构建请求
type LLMPromptRequest struct {
	Symbol  string   `json:"symbol" validate:"required"`
	NewsIDs []string `json:"news_ids"`
}

// LLMAnalysisRequest LLM流式分析请求
type LLMAnalysisRequest struct {
	Prompt string `json:"prompt" validate:"required"`
}

// AnalyzeSymbol 分析交易对（技术分析）
func (h *MarketHandler) AnalyzeSymbol(c echo.Context) error {
	var req MarketAnalysisRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	if req.Limit <= 0 {
		req.Limit = 100
	}

	// 获取K线数据
	klines, err := h.binanceService.GetKlineData(req.Symbol, req.Interval, req.Limit)
	if err != nil {
		return err
	}

	// 转换为内部模型
	klineData, err := h.binanceService.ConvertKlineToModels(klines)
	if err != nil {
		return err
	}

	// 计算技术指标（服务端计算）
	indicators, err := h.technicalService.CalculateIndicators(klineData)
	if err != nil {
		return err
	}

	// 进行市场分析
	analysis, err := h.technicalService.AnalyzeMarket(req.Symbol, klineData, indicators)
	if err != nil {
		return err
	}

	analysis.Symbol = req.Symbol

	return c.JSON(http.StatusOK, map[string]interface{}{
		"symbol":               req.Symbol,
		"interval":             req.Interval,
		"analysis":             analysis,
		"technical_indicators": indicators,
	})
}

// GetKlineData 获取K线数据
func (h *MarketHandler) GetKlineData(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	interval := c.QueryParam("interval")
	limitStr := c.QueryParam("limit")

	if symbol == "" || interval == "" {
		return fmt.Errorf("missing required parameters: symbol, interval")
	}

	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取K线数据
	klines, err := h.binanceService.GetKlineData(symbol, interval, limit)
	if err != nil {
		return err
	}

	// 转换为内部模型
	klineData, err := h.binanceService.ConvertKlineToModels(klines)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"symbol":     symbol,
		"interval":   interval,
		"kline_data": klineData,
	})
}

// GetSymbols 从缓存返回币安交易对列表（TRADING），避免频繁请求
func (h *MarketHandler) GetSymbols(c echo.Context) error {
	symbols, err := h.binanceService.GetCachedSymbols()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"symbols": symbols,
	})
}

// GetMarketOverview 获取市场概览
func (h *MarketHandler) GetMarketOverview(c echo.Context) error {
	summary, err := h.binanceService.GetMarketSummary()
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, summary)
}

// GetTrendingSymbols 获取热门交易对
func (h *MarketHandler) GetTrendingSymbols(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	limit := 5
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 20 {
			limit = l
		}
	}

	trending, err := h.binanceService.GetTopSymbols(limit)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, trending)
}

func (h *MarketHandler) getTimeline(symbol, interval string, limit int) (*models.Timeline, error) {
	// K线 + 技术指标
	klines, err := h.binanceService.GetKlineData(symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %v", err)
	}
	klineData, err := h.binanceService.ConvertKlineToModels(klines)
	if err != nil {
		return nil, fmt.Errorf("转换K线数据失败: %v", err)
	}
	indicators, err := h.technicalService.CalculateIndicators(klineData)
	if err != nil {
		return nil, fmt.Errorf("计算技术指标失败: %v", err)
	}
	timeline := &models.Timeline{
		Data:       klineData,
		Indicators: indicators,
		Interval:   interval,
	}
	return timeline, nil
}

// BuildLLMPrompt 构建LLM分析提示词
func (h *MarketHandler) BuildLLMPrompt(c echo.Context) error {
	ctx := c.Request().Context()

	var req LLMPromptRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request parameters",
		})
	}

	// 获取合约数据
	fundingRate, err := h.binanceService.GetFundingRate(req.Symbol)
	if err != nil {
		h.logger.Warn("获取资金费率失败", zap.Error(err))
		fundingRate = "N/A"
	}

	longShortRatio, err := h.binanceService.GetLongShortRatio(req.Symbol, "5m")
	if err != nil {
		h.logger.Warn("获取多空比失败", zap.Error(err))
		longShortRatio = "N/A"
	}

	// 获取多时间轴数据
	hourlyTimeline, err := h.getTimeline(req.Symbol, "1h", 200)
	if err != nil {
		h.logger.Error("获取小时线数据失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch hourly timeline",
		})
	}

	dailyTimeline, err := h.getTimeline(req.Symbol, "1d", 180)
	if err != nil {
		h.logger.Error("获取日线数据失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch daily timeline",
		})
	}

	// 获取选中的新闻
	var selectedNews []models.News
	if len(req.NewsIDs) > 0 {
		selectedNews, err = h.newsService.FindByIdIn(ctx, req.NewsIDs)
		if err != nil {
			h.logger.Warn("获取新闻失败", zap.Error(err))
		}
	}

	timelines := &models.Timelines{
		Symbol:         req.Symbol,
		Daily:          dailyTimeline,
		Hourly:         hourlyTimeline,
		News:           selectedNews,
		FundingRate:    fundingRate,
		LongShortRatio: longShortRatio,
	}

	// 构建优化的提示词
	prompt := h.llmService.BuildOptimizedPrompt(timelines)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"symbol": req.Symbol,
		"prompt": prompt,
	})
}

// LLMAnalyzeStream 使用LLM进行流式分析
func (h *MarketHandler) LLMAnalyzeStream(c echo.Context) error {
	var req LLMAnalysisRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request parameters",
		})
	}

	// 设置SSE响应头
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	// SSE消息发送器
	sendSSEMessage := func(eventType, content string) error {
		message := map[string]string{
			"type":    eventType,
			"content": content,
		}
		if eventType == "error" {
			message["message"] = content
			message["content"] = ""
		}
		data, _ := json.Marshal(message)
		_, err := fmt.Fprintf(c.Response(), "data: %s\n\n", data)
		if err != nil {
			return err
		}
		c.Response().Flush()
		return nil
	}

	// 执行LLM流式分析
	if err := h.llmService.AnalyzePromptStream(ctx, req.Prompt, sendSSEMessage); err != nil {
		h.logger.Error("LLM流式分析失败", zap.Error(err))
		sendSSEMessage("error", "Failed to perform LLM stream analysis")
		return nil
	}

	sendSSEMessage("done", "")
	return nil
}
