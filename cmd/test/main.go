package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(`请分析以下加密货币新闻文本的情绪倾向，并以 JSON 格式返回结果，包含以下三个字段：
- "score"：情绪得分，范围从 -1.0（极度利空）到 1.0（极度利好），用浮点数表示。
- "sentiment"：情绪类别，只能是："利好"、"利空" 或 "中性"。
- "summary"：对新闻内容的简要总结，100个字以内，使用中文。

请**仅输出 JSON 对象**，不要包含任何解释、说明或其他文字。`, genai.RoleModel),
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-pro",
		genai.Text(`美国缉毒局在卧底行动中处理1900万美元贩毒资金，其中部分流入加密货币 【美国缉毒局在卧底行动中处理1900万美元贩毒资金，其中部分流入加密货币】金色财经报道，美国缉毒局（DEA）在一项长达十余年的卧底行动中，成功渗透并控制了哥伦比亚贩毒集团的洗钱网络，处理资金总额约1900万美元，其中部分资金被转入加密货币账户。行动最终导致两名涉海洛因贩运人员被起诉。 \n相关文件显示，2018年DEA卧底人员曾将15万美元转入Coinbase账户并兑换为超13枚比特币，受益于价格上涨，这部分资产目前价值已超150万美元。司法部正寻求追回相关资金。（福布斯）`),
		config,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Text())
}
