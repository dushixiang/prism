package main

import (
	"fmt"
	"os"

	"github.com/dushixiang/prism/internal"
	"github.com/dushixiang/prism/internal/cli"
	"github.com/spf13/cobra"
)

var (
	configFile string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "prism",
		Short: "Prism - 加密货币分析平台",
		Long: `Prism 是一个专业的加密货币分析平台，
提供技术分析、大模型分析、新闻监控等功能。`,
	}

	// 全局配置文件标志
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "配置文件路径")

	// 服务器启动命令
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 Prism 服务器",
		Long:  `启动 Prism Web 服务器，提供 API 和前端界面`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return internal.Run(configFile)
		},
	}

	// 用户管理命令
	userCmd := cli.NewUserCommand(configFile)

	// 添加子命令
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(userCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
