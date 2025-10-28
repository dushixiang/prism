package main

import (
	"log"

	"github.com/dushixiang/prism/internal"
	"github.com/spf13/cobra"
)

var (
	configFile string
)

var rootCmd = &cobra.Command{
	Use:   "prism",
	Short: "Prism - 加密货币AI自动交易系统",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.Run(configFile)
	},
}

func init() {
	// 全局配置文件标志
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "配置文件路径")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
