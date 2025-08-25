package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/models"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/valyala/fasttemplate"
	"go.uber.org/zap"
)

// LLMAnalysisService 大模型分析服务
type LLMAnalysisService struct {
	config *config.Config
	logger *zap.Logger
	client *openai.Client
}

// NewLLMAnalysisService 创建新的大模型分析服务实例
func NewLLMAnalysisService(config *config.Config, logger *zap.Logger) *LLMAnalysisService {
	client := openai.NewClient(
		option.WithBaseURL(config.LLM.BaseURL),
		option.WithAPIKey(config.LLM.APIKey),
	)

	return &LLMAnalysisService{
		config: config,
		logger: logger,
		client: &client,
	}
}

// BuildOptimizedPrompt 构建优化的提示词（基于多时间轴 + 指标 + 市场要素 + 新闻）
func (s *LLMAnalysisService) BuildOptimizedPrompt(timelines *models.Timelines) string {
	if timelines == nil {
		return "数据为空"
	}

	symbol := strings.ToUpper(timelines.Symbol)
	// 提取小时/日线数据
	var hourlyLast, hourlyPrev *models.KlineData
	if timelines.Hourly != nil && len(timelines.Hourly.Data) > 0 {
		data := timelines.Hourly.Data
		hourlyLast = &data[len(data)-1]
		if len(data) > 1 {
			hourlyPrev = &data[len(data)-2]
		}
	}
	var dailyLast *models.KlineData
	if timelines.Daily != nil && len(timelines.Daily.Data) > 0 {
		d := timelines.Daily.Data
		dailyLast = &d[len(d)-1]
	}

	// 基础价格信息（优先使用小时线的最后一根）
	currentPrice := 0.0
	highPrice := 0.0
	lowPrice := 0.0
	volume := 0.0

	if hourlyLast != nil {
		currentPrice = hourlyLast.ClosePrice
		volume = hourlyLast.Volume

		// 取最近 24 根小时线
		hourlyData := timelines.Hourly.Data
		if len(hourlyData) > 24 {
			hourlyData = hourlyData[len(hourlyData)-24:]
		}

		// 初始化 24 小时最高/最低价
		highPrice = hourlyData[0].HighPrice
		lowPrice = hourlyData[0].LowPrice

		for _, k := range hourlyData {
			if k.HighPrice > highPrice {
				highPrice = k.HighPrice
			}
			if k.LowPrice < lowPrice {
				lowPrice = k.LowPrice
			}
		}
	} else if dailyLast != nil {
		currentPrice = dailyLast.ClosePrice
		highPrice = dailyLast.HighPrice
		lowPrice = dailyLast.LowPrice
		volume = dailyLast.Volume
	}

	priceChange := 0.0
	priceChangePercent := 0.0
	if hourlyLast != nil && hourlyPrev != nil && hourlyPrev.ClosePrice != 0 {
		priceChange = hourlyLast.ClosePrice - hourlyPrev.ClosePrice
		priceChangePercent = priceChange / hourlyPrev.ClosePrice * 100
	}

	// 指标（安全读取）
	hInd := safeIndicators(timelines.Hourly)
	dInd := safeIndicators(timelines.Daily)

	// 新闻（情绪+分数） — 限制长度，避免提示过长
	newsSummary := s.formatNewsItems(timelines.News, 10)

	// 模板
	t := fasttemplate.New(`你是一名资深的加密货币分析师，精通技术分析、市场情绪解读和宏观事件影响。请基于以下多维度数据，输出 {{symbol}} 的短期（1小时内）与中期（1天内）走势研判。

## 核心市场数据
- 分析标的：{{symbol}}
- 当前时间：{{now}}
- 主流交易所多空比：{{long_short_ratio}}
- 永续合约资金费率：{{funding_rate}}

### 价格概况（最近一根K线）
- 当前价格：${{current_price}}
- 价格变化：${{price_change}} ({{price_change_percent}}%)
- 最近24小时最高 / 最低：${{high_price}} / ${{low_price}}
- 成交量：{{volume}}

### 多时间周期K线
- 1小时：
{{hourly_kline}}
- 日线：
{{daily_kline}}

### 技术指标
- 1小时
  - MA5/20/50：${{hourly_ma5}} / ${{hourly_ma20}} / ${{hourly_ma50}}
  - RSI：{{hourly_rsi}}
  - MACD（DIF/DEA/HIST）：{{hourly_macd}} / {{hourly_macd_signal}} / {{hourly_macd_hist}}
  - 布林带（上/中/下）：{{hourly_bb_upper}} / {{hourly_bb_middle}} / {{hourly_bb_lower}}
  - SAR：{{hourly_sar}}
- 日线
  - MA5/20/50：${{daily_ma5}} / ${{daily_ma20}} / ${{daily_ma50}}
  - RSI：{{daily_rsi}}
  - MACD（DIF/DEA/HIST）：{{daily_macd}} / {{daily_macd_signal}} / {{daily_macd_hist}}
  - 布林带（上/中/下）：{{daily_bb_upper}} / {{daily_bb_middle}} / {{daily_bb_lower}}
  - SAR：{{daily_sar}}

### 相关新闻摘要（利好/利空/中性，含情绪评分）
{{news_block}}

## 任务与输出要求
- 综合分析：结合技术面、情绪面与事件驱动，判断短期与中期的主导因素与风险点。
- 关键价位：给出明确的支撑/阻力位与触发条件。
- 风险评估：识别主要风险并说明可能触发路径。

请按以下结构输出：
**分析总结** 
**价格趋势** 
**技术分析** 
**支撑阻力位** 
**价格预测**（短期/中期/趋势方向） 
**风险评估** 
**策略思路与风险管理**
`, "{{", "}}")

	base := t.ExecuteString(map[string]interface{}{
		"symbol":               symbol,
		"now":                  time.Now().Format(time.DateTime),
		"long_short_ratio":     emptyIf(timelines.LongShortRatio),
		"funding_rate":         emptyIf(timelines.FundingRate),
		"current_price":        fmt.Sprintf("%.4f", currentPrice),
		"price_change":         fmt.Sprintf("%.4f", priceChange),
		"price_change_percent": fmt.Sprintf("%.2f", priceChangePercent),
		"high_price":           fmt.Sprintf("%.4f", highPrice),
		"low_price":            fmt.Sprintf("%.4f", lowPrice),
		"volume":               fmt.Sprintf("%.2f", volume),
		"hourly_kline":         s.formatKlineData(getKline(timelines.Hourly)),
		"daily_kline":          s.formatKlineData(getKline(timelines.Daily)),
		// 小时指标
		"hourly_ma5":         fmt.Sprintf("%.4f", hInd.MA5),
		"hourly_ma20":        fmt.Sprintf("%.4f", hInd.MA20),
		"hourly_ma50":        fmt.Sprintf("%.4f", hInd.MA50),
		"hourly_rsi":         fmt.Sprintf("%.2f", hInd.RSI),
		"hourly_macd":        fmt.Sprintf("%.4f", hInd.MACD),
		"hourly_macd_signal": fmt.Sprintf("%.4f", hInd.MACDSignal),
		"hourly_macd_hist":   fmt.Sprintf("%.4f", hInd.MACDHist),
		"hourly_bb_upper":    fmt.Sprintf("%.4f", hInd.BBUpper),
		"hourly_bb_middle":   fmt.Sprintf("%.4f", hInd.BBMiddle),
		"hourly_bb_lower":    fmt.Sprintf("%.4f", hInd.BBLower),
		"hourly_sar":         fmt.Sprintf("%.4f", hInd.SAR),
		// 日线指标
		"daily_ma5":         fmt.Sprintf("%.4f", dInd.MA5),
		"daily_ma20":        fmt.Sprintf("%.4f", dInd.MA20),
		"daily_ma50":        fmt.Sprintf("%.4f", dInd.MA50),
		"daily_rsi":         fmt.Sprintf("%.2f", dInd.RSI),
		"daily_macd":        fmt.Sprintf("%.4f", dInd.MACD),
		"daily_macd_signal": fmt.Sprintf("%.4f", dInd.MACDSignal),
		"daily_macd_hist":   fmt.Sprintf("%.4f", dInd.MACDHist),
		"daily_bb_upper":    fmt.Sprintf("%.4f", dInd.BBUpper),
		"daily_bb_middle":   fmt.Sprintf("%.4f", dInd.BBMiddle),
		"daily_bb_lower":    fmt.Sprintf("%.4f", dInd.BBLower),
		"daily_sar":         fmt.Sprintf("%.4f", dInd.SAR),
		"news_block":        newsSummary,
	})
	return base
}

func emptyIf(v string) string {
	if strings.TrimSpace(v) == "" {
		return "N/A"
	}
	return v
}

func getKline(t *models.Timeline) []models.KlineData {
	if t == nil {
		return nil
	}
	return t.Data
}

func safeIndicators(t *models.Timeline) *models.TechnicalIndicators {
	if t == nil || t.Indicators == nil {
		return &models.TechnicalIndicators{}
	}
	return t.Indicators
}

// formatKlineData 格式化K线数据为易读格式（限制行数，补全代码块围栏）
func (s *LLMAnalysisService) formatKlineData(klineData []models.KlineData) string {
	if len(klineData) == 0 {
		return "无K线数据"
	}

	var b strings.Builder
	b.WriteString("```text\n")
	b.WriteString("时间\t\t开盘\t最高\t最低\t收盘\t成交量\n")
	b.WriteString("─────────────────────────────────────────────────\n")

	rowTemplate := fasttemplate.New("{{time}}\t${{open}}\t${{high}}\t${{low}}\t${{close}}\t{{volume}}\n", "{{", "}}")
	for i := 0; i < len(klineData); i++ {
		kline := klineData[i]
		timeStr := kline.OpenTime.Format("01-02 15:04")
		priceFormat := "%.4f"
		if kline.ClosePrice >= 1000 {
			priceFormat = "%.2f"
		} else if kline.ClosePrice >= 100 {
			priceFormat = "%.3f"
		}
		b.WriteString(rowTemplate.ExecuteString(map[string]interface{}{
			"time":   timeStr,
			"open":   fmt.Sprintf(priceFormat, kline.OpenPrice),
			"high":   fmt.Sprintf(priceFormat, kline.HighPrice),
			"low":    fmt.Sprintf(priceFormat, kline.LowPrice),
			"close":  fmt.Sprintf(priceFormat, kline.ClosePrice),
			"volume": fmt.Sprintf("%.2f", kline.Volume),
		}))
	}
	b.WriteString("```")
	return b.String()
}

func (s *LLMAnalysisService) formatNewsItems(items []models.News, max int) string {
	if len(items) == 0 {
		return "(无近期重要新闻)"
	}
	if max <= 0 {
		max = 5
	}
	toZh := func(sent string) string {
		s := strings.ToLower(sent)
		switch s {
		case "positive":
			return "利好"
		case "negative":
			return "利空"
		default:
			return "中性"
		}
	}
	var b strings.Builder
	for i, n := range items {
		if i >= max {
			break
		}
		score := fmt.Sprintf("%.2f", n.Score)
		b.WriteString(fmt.Sprintf("- [%s | 评分:%s] %s — %s\n", toZh(n.Sentiment), score, n.Summary, n.Source))
	}
	return b.String()
}

// callLLMService 调用AI服务
func (s *LLMAnalysisService) callLLMService(prompt string) (string, error) {
	model := s.config.LLM.Model.Default
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		chatCompletion, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(prompt)},
			Model:    model,
		})
		cancel()
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(chatCompletion.Choices) == 0 {
			lastErr = fmt.Errorf("AI响应为空")
			time.Sleep(200 * time.Millisecond)
			continue
		}
		content := chatCompletion.Choices[0].Message.Content
		if content == "" {
			lastErr = fmt.Errorf("AI响应内容为空")
			continue
		}
		return content, nil
	}
	return "", fmt.Errorf("调用OpenAI失败: %v", lastErr)
}

func (s *LLMAnalysisService) AnalyzePromptStream(
	ctx context.Context,
	prompt string,
	sendMessage func(eventType, content string) error,
) error {
	model := s.config.LLM.Model.Default
	// 继承上游ctx，由上层控制超时
	stream := s.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(prompt)},
		Model:    model,
	})

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			if err := sendMessage("content", content); err != nil {
				return fmt.Errorf("发送消息失败: %v", err)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("调用OpenAI流式API失败: %v", err)
	}
	return nil
}

type Sentiment struct {
	Score     float64 `json:"score"`
	Sentiment string  `json:"sentiment"` // positive: 积极 | negative: 消极 | neutral: 中立
	Summary   string  `json:"summary"`   // 简要总结，100字以内
}

const SentimentPrompt = `请分析以下加密货币新闻文本的情绪倾向，并以 JSON 格式返回结果，包含以下三个字段：
- "score"：情绪得分，范围从 -1.0（极度利空）到 1.0（极度利好），用浮点数表示。
- "sentiment"：情绪类别，只能是："利好"、"利空" 或 "中性"。
- "summary"：对新闻内容的简要总结，100个字以内，使用中文。

请**仅输出 JSON 对象**，不要包含任何解释、说明或其他文字。`

func (s *LLMAnalysisService) SentimentAnalysis(content string) (*Sentiment, error) {
	var model = s.config.LLM.Model.SentimentAnalysis
	if model == "" {
		model = s.config.LLM.Model.Default
	}

	chatCompletion, err := s.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(SentimentPrompt),
			openai.UserMessage(content),
		},
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %v", err)
	}
	var response = chatCompletion.Choices[0].Message.Content
	response = stripCodeFences(response)
	var result Sentiment
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		s.logger.Warn("解析 LLM 返回结果失败，使用文本形式", zap.String("response", response), zap.Error(err))
		return nil, fmt.Errorf("解析 LLM 返回结果失败: %v", err)
	}

	switch result.Sentiment {
	case "利好":
		result.Sentiment = "positive"
	case "利空":
		result.Sentiment = "negative"
	case "中性":
		result.Sentiment = "neutral"
	}

	return &result, nil
}

// stripCodeFences 去除```json/``` 包裹
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// 去掉第一行```... 和最后的```
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			// 去掉首尾
			lines = lines[1:]
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			s = strings.Join(lines, "\n")
		}
	}
	return strings.TrimSpace(s)
}
