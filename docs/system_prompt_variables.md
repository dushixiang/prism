# 系统提示词变量说明

系统提示词模板位于 `internal/service/templates/system_instructions.tmpl`，通过 `github.com/valyala/fasttemplate` 渲染生成最终文案。以下列出模板中涉及的占位符及含义：

- `{{minutes_elapsed}}`：自策略启动至今经过的分钟数，由运行时根据 `PromptData.StartTime` 动态计算。
- `{{current_time}}`：生成提示词时的当前时间（中国时区），用于同步模型与实时市场环境。
- `{{iteration_count}}`：当前是第几次调用模型，可帮助追踪历史对话与迭代次数。
- `{{max_drawdown_percent}}`：最大允许回撤百分比，来自配置项 `trading.max_drawdown_percent`。
- `{{forced_flat_percent}}`：强制平仓回撤阈值，等于 `max_drawdown_percent + 5`，用于触发全面风控。
- `{{max_positions}}`：允许的最大持仓数量，来自配置项 `trading.max_positions`。
- `{{risk_percent_per_trade}}`：单笔交易允许承担的账户风险百分比，来自配置项 `trading.risk_percent_per_trade`。
- `{{low_leverage_range}}` / `{{mid_leverage_range}}` / `{{high_leverage_range}}`：根据配置的最小与最大杠杆划分的三个建议区间，用于指导模型选择杠杆强度。

其中与账户和时间相关的变量（如 `{{minutes_elapsed}}`、`{{current_time}}`、`{{iteration_count}}`）在系统指令中保持占位形式，便于后续生成完整提示词时统一替换；而与配置相关的变量会在服务器启动时通过模板渲染直接替换为具体数值。
