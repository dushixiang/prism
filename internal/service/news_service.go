package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/dushixiang/prism/internal/models"
	"github.com/dushixiang/prism/internal/repo"
	"github.com/go-orz/orz"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewNewsService(logger *zap.Logger, db *gorm.DB, llmAnalysisService *LLMAnalysisService) *NewsService {
	service := &NewsService{
		Service:  orz.NewService(db),
		logger:   logger,
		NewsRepo: repo.NewNewsRepo(db),
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
		sources:            make(map[string]*NewsSource),
		llmAnalysisService: llmAnalysisService,
	}

	return service
}

// RSS RSS结构体定义
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title string `xml:"title"`
	Items []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// NewsSource 新闻源配置
type NewsSource struct {
	Name     string
	URL      string
	Parser   func([]byte) ([]models.News, error)
	Interval time.Duration
}

type NewsService struct {
	logger *zap.Logger
	*orz.Service
	*repo.NewsRepo

	httpClient *http.Client
	sources    map[string]*NewsSource

	llmAnalysisService *LLMAnalysisService
}

func (r *NewsService) StartSubscriber() {
	// 初始化新闻源
	r.initSources()
	r.startSubscriber()
}

// initSources 初始化新闻源
func (r *NewsService) initSources() {
	// 金色财经
	r.sources["jinse"] = &NewsSource{
		Name:     "金色财经",
		URL:      "https://api.jinse.cn/noah/v2/lives?limit=20&reading=false&source=web&flag=up&id=0&category=0",
		Parser:   r.parseJinseNews,
		Interval: time.Second * 10,
	}

	// CoinDesk
	r.sources["coindesk"] = &NewsSource{
		Name:     "CoinDesk",
		URL:      "https://www.coindesk.com/arc/outboundfeeds/rss/",
		Parser:   r.parseCoinDeskNews,
		Interval: time.Second * 15,
	}

	// Cointelegraph
	r.sources["cointelegraph"] = &NewsSource{
		Name:     "Cointelegraph",
		URL:      "https://cointelegraph.com/rss",
		Parser:   r.parseCointelegraphNews,
		Interval: time.Second * 15,
	}
}

func (r *NewsService) startSubscriber() {
	// 为每个新闻源启动独立的goroutine
	for _, source := range r.sources {
		go r.subscribeSource(source)
	}
}

// subscribeSource 订阅单个新闻源
func (r *NewsService) subscribeSource(source *NewsSource) {
	r.logger.Info("启动新闻源监控", zap.String("source", source.Name))

	for {
		// 随机间隔避免被反爬
		baseInterval := source.Interval
		jitter := time.Duration(rand.Intn(5)) * time.Second
		time.Sleep(baseInterval + jitter)

		news, err := r.fetchFromSource(source)
		if err != nil {
			r.logger.Error("获取新闻失败",
				zap.String("source", source.Name),
				zap.Error(err))
			continue
		}

		if len(news) == 0 {
			continue
		}

		// 保存新闻
		savedCount := 0
		for _, n := range news {
			// 检查新闻是否已存在
			if r.isNewsExists(n.OriginalID, source.Name) {
				continue
			}
			n.Source = source.Name

			// 调用LLM进行情绪分析
			sentiment, err := r.llmAnalysisService.SentimentAnalysis(n.Title + " " + n.Content)
			if err != nil {
				r.logger.Error("LLM分析失败",
					zap.Error(err),
					zap.String("title", n.Title),
					zap.String("content", n.Content),
					zap.String("news_id", n.ID),
					zap.String("source", source.Name))
			} else {
				n.Sentiment = sentiment.Sentiment
				n.Score = sentiment.Score
				n.Summary = sentiment.Summary
			}

			// 保存新闻
			if err := r.saveNews(n); err != nil {
				r.logger.Error("保存新闻失败",
					zap.Error(err),
					zap.String("news_id", n.ID),
					zap.String("source", source.Name))
				continue
			}

			savedCount++
		}

		if savedCount > 0 {
			r.logger.Info("保存新闻成功",
				zap.String("source", source.Name),
				zap.Int("count", savedCount))
		}
	}
}

// fetchFromSource 从指定新闻源获取数据
func (r *NewsService) fetchFromSource(source *NewsSource) ([]models.News, error) {
	if source.Name == "金色财经" {
		news, err := r.FindBySource(context.Background(), source.Name, 1)
		if err != nil {
			return nil, err
		}
		if len(news) > 0 {
			source.URL = fmt.Sprintf("https://api.jinse.cn/noah/v2/lives?limit=20&reading=false&source=web&flag=up&id=%s&category=0", news[0].OriginalID)
		}
	}
	request, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return nil, err
	}

	// 设置不同的User-Agent避免被封
	userAgents := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	}
	request.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])

	response, err := r.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("获取新闻失败: %s, 状态码: %d", source.Name, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return source.Parser(body)
}

// parseJinseNews 解析金色财经新闻
func (r *NewsService) parseJinseNews(body []byte) ([]models.News, error) {
	var news []models.News
	gson := gjson.ParseBytes(body)
	lives := gson.Get("list.0.lives").Array()

	for _, live := range lives {
		originalID := fmt.Sprintf("%d", live.Get("id").Int())
		title := live.Get("content_prefix").String()
		content := live.Get("content").String()
		createdAt := live.Get("created_at").Int()
		link := live.Get("link").String()

		news = append(news, models.News{
			ID:         uuid.New().String(),
			Title:      title,
			Content:    content,
			Link:       link,
			CreatedAt:  createdAt * 1000,
			OriginalID: originalID,
		})
	}
	return news, nil
}

// parseCoinDeskNews 解析CoinDesk RSS新闻
func (r *NewsService) parseCoinDeskNews(body []byte) ([]models.News, error) {
	var rss RSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("解析CoinDesk RSS失败: %w", err)
	}

	var news []models.News
	for _, item := range rss.Channel.Items {
		// 解析发布时间
		pubTime, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			// 尝试其他时间格式
			pubTime, err = time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", item.PubDate)
			if err != nil {
				pubTime = time.Now() // 使用当前时间作为fallback
			}
		}

		news = append(news, models.News{
			ID:         uuid.New().String(),
			Title:      item.Title,
			Content:    item.Description,
			Link:       item.Link,
			CreatedAt:  pubTime.Unix() * 1000,
			OriginalID: item.GUID,
		})
	}

	return news, nil
}

// parseCointelegraphNews 解析Cointelegraph RSS新闻
func (r *NewsService) parseCointelegraphNews(body []byte) ([]models.News, error) {
	var rss RSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("解析Cointelegraph RSS失败: %w", err)
	}

	var news []models.News
	for _, item := range rss.Channel.Items {
		// 解析发布时间
		pubTime, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			// 尝试其他时间格式
			pubTime, err = time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", item.PubDate)
			if err != nil {
				pubTime = time.Now() // 使用当前时间作为fallback
			}
		}

		news = append(news, models.News{
			ID:         uuid.New().String(),
			Title:      item.Title,
			Content:    item.Description,
			Link:       item.Link,
			CreatedAt:  pubTime.Unix() * 1000,
			OriginalID: item.GUID,
		})
	}

	return news, nil
}

// isNewsExists 检查新闻是否已存在（基于原始ID和来源）
func (r *NewsService) isNewsExists(originalID, source string) bool {
	ctx := context.Background()
	exists, err := r.NewsRepo.ExistsByOriginalIDAndSource(ctx, originalID, source)
	if err != nil {
		r.logger.Error("检查新闻是否存在时出错", zap.Error(err))
		return false
	}
	return exists
}

// saveNews 保存新闻到数据库
func (r *NewsService) saveNews(news models.News) error {
	ctx := context.Background()
	return r.NewsRepo.Create(ctx, &news)
}

// GetLatestNews 获取最新新闻列表
func (r *NewsService) GetLatestNews(limit int) ([]models.News, error) {
	if limit <= 0 {
		limit = 20
	}
	ctx := context.Background()
	return r.NewsRepo.FindLatest(ctx, limit)
}

// GetNewsByTimeRange 根据时间范围获取新闻
func (r *NewsService) GetNewsByTimeRange(startTime, endTime int64) ([]models.News, error) {
	ctx := context.Background()
	return r.NewsRepo.FindByTimeRange(ctx, startTime, endTime)
}

// GetNewsStatistics 获取新闻统计信息
func (r *NewsService) GetNewsStatistics() (map[string]interface{}, error) {
	ctx := context.Background()

	// 获取总新闻数量
	totalNews, err := r.NewsRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	// 获取今日新闻数量
	today := time.Now()
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).Unix() * 1000
	todayEnd := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 999, today.Location()).Unix() * 1000

	todayCount, err := r.NewsRepo.CountToday(ctx, todayStart, todayEnd)
	if err != nil {
		todayCount = 0
	}

	// 按新闻源统计
	var sourceStats []map[string]interface{}
	for _, source := range r.sources {
		count, err := r.NewsRepo.CountBySource(ctx, source.Name)
		if err != nil {
			count = 0
		}

		sourceStats = append(sourceStats, map[string]interface{}{
			"name":  source.Name,
			"count": count,
		})
	}

	// 按情绪统计
	positiveCount, err := r.NewsRepo.CountBySentiment(ctx, "positive")
	if err != nil {
		positiveCount = 0
	}

	negativeCount, err := r.NewsRepo.CountBySentiment(ctx, "negative")
	if err != nil {
		negativeCount = 0
	}

	neutralCount, err := r.NewsRepo.CountBySentiment(ctx, "neutral")
	if err != nil {
		neutralCount = 0
	}

	stats := map[string]interface{}{
		"total_news": totalNews,
		"today_news": todayCount,
		"sources":    sourceStats,
		"sentiment": map[string]int64{
			"positive": positiveCount,
			"negative": negativeCount,
			"neutral":  neutralCount,
		},
		"last_updated": time.Now().Unix(),
	}

	return stats, nil
}
