package cli

import (
	"fmt"
	"time"

	"github.com/dushixiang/prism/internal"
	"github.com/dushixiang/prism/internal/config"
	"github.com/dushixiang/prism/internal/ioc"
	"github.com/go-orz/orz"
)

// initializeContainer 初始化容器和服务
func initializeContainer(configFile string) (*ioc.Container, error) {
	// 使用 orz 框架初始化组件
	framework, err := orz.NewFramework(
		orz.WithConfig(configFile),
		orz.WithLoggerFromConfig(),
		orz.WithDatabase(),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化框架失败: %v", err)
	}

	// 获取日志器
	logger := framework.App().Logger()

	// 获取数据库
	db, err := framework.App().GetDatabase()
	if err != nil {
		return nil, fmt.Errorf("获取数据库失败: %v", err)
	}

	// 获取配置
	var conf config.Config
	err = framework.App().GetConfig().App.Unmarshal(&conf)
	if err != nil {
		return nil, fmt.Errorf("解析配置失败: %v", err)
	}

	// 创建容器 - 调用 internal 包中的 ProviderContainer
	container := internal.ProviderContainer(logger, db, &conf)

	return container, nil
}

// formatTimestamp 格式化时间戳
func formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return "N/A"
	}
	return time.UnixMilli(timestamp).Format("2006-01-02 15:04:05")
}
