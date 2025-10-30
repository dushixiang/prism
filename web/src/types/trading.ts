export type TradingLoopStatus = {
    is_running: boolean;
    iteration: number;
    start_time: string;
    elapsed_hours: number;
    symbols: string[];
    interval_minutes: number;
};

export type AccountMetrics = {
    total_balance: number;
    available: number;
    unrealised_pnl: number;
    initial_balance: number;
    peak_balance: number;
    return_percent: number;
    drawdown_from_peak: number;
    drawdown_from_initial: number;
    sharpe_ratio: number;
    warnings?: string[];
};

export type Position = {
    id: string;
    symbol: string;
    side: string;
    quantity: number;
    entry_price: number;
    current_price?: number;
    liquidation_price?: number;
    unrealized_pnl?: number;
    pnl_percent?: number;
    leverage?: number;
    margin?: number;
    peak_pnl_percent?: number;
    holding?: string;
    opened_at?: string;
    warnings?: string[];
    entry_reason?: string;
    exit_plan?: string;
};

export type Decision = {
    id: string;
    iteration: number;
    account_value: number;
    position_count: number;
    decision_content: string;
    prompt_tokens: number;
    completion_tokens: number;
    model: string;
    executed_at: string;
};

export type LLMLog = {
    id: string;
    decision_id: string;
    iteration: number;
    round_number: number;
    model: string;
    system_prompt: string;
    user_prompt: string;
    messages: string;
    assistant_content: string;
    tool_calls: string;
    tool_responses: string;
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
    finish_reason: string;
    duration: number;
    error: string;
    executed_at: string;
};

export type LLMLogsResponse = {
    logs: LLMLog[];
};

export type Trade = {
    id: string;
    symbol: string;
    type: string;
    side: string;
    price: number;
    quantity: number;
    leverage: number;
    fee: number;
    pnl: number;
    executed_at: string;
};

export type TradingStatusResponse = {
    loop: TradingLoopStatus;
    account?: AccountMetrics;
    positions?: Position[];
};

export type AccountResponse = AccountMetrics;

export type PositionsResponse = {
    count: number;
    positions: Position[];
};

export type DecisionsResponse = {
    count: number;
    decisions: Decision[];
};

export type TradesResponse = {
    count: number;
    trades: Trade[];
};

export type EquityCurveDataPoint = {
    timestamp: number;
    time: string;
    total_balance: number;
    available: number;
    unrealised_pnl: number;
    return_percent: number;
    drawdown_from_peak: number;
    drawdown_from_initial: number;
    iteration: number;
};

export type EquityCurveResponse = {
    count: number;
    data: EquityCurveDataPoint[];
};
