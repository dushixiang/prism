package handler

import (
	"net/http"

	"github.com/dushixiang/prism/internal/service"
	"github.com/go-orz/orz"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

type NewsHandler struct {
	newsService *service.NewsService
	logger      *zap.Logger
}

func NewNewsHandler(newsService *service.NewsService, logger *zap.Logger) *NewsHandler {
	return &NewsHandler{
		newsService: newsService,
		logger:      logger,
	}
}

// GetLatestNews 获取最新新闻
func (h *NewsHandler) GetLatestNews(c echo.Context) error {
	limit := cast.ToInt(c.QueryParam("limit"))
	if limit == 0 {
		limit = 20
	}
	source := c.QueryParam("source")
	sentiment := c.QueryParam("sentiment")
	keyword := c.QueryParam("keyword")

	builder := orz.NewPageBuilder(h.newsService.NewsRepo.Repository).
		PageSize(limit).
		Equal("source", source).
		Equal("sentiment", sentiment).
		Keyword([]string{"title", "content", "summary"}, keyword).
		SortByDesc("CreatedAt", "CreatedAt")

	ctx := c.Request().Context()
	result, err := builder.Execute(ctx)
	if err != nil {
		return err
	}

	return orz.Ok(c, result.Items)
}

// GetNewsStatistics 获取新闻统计信息
func (h *NewsHandler) GetNewsStatistics(c echo.Context) error {
	stats, err := h.newsService.GetNewsStatistics()
	if err != nil {
		h.logger.Error("获取新闻统计信息失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch news statistics",
		})
	}

	return orz.Ok(c, stats)
}
