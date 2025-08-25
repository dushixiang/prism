// ===== 错误响应类型 =====
export interface ErrorResponse {
    code: number;
    message: string;
}

// ===== 用户认证相关类型 =====
export interface LoginRequest {
    account: string;
    password: string;
}

export interface LoginResponse {
    token: string;
}

export interface UserInfo {
    id: string;
    name: string;
    account: string;
    avatar: string;
    createdAt: number;
    enabled: boolean;
    type: string;
}

// ===== 市场数据类型 =====
export interface KlineData {
    open_time: string;
    close_time: string;
    open_price: number;
    high_price: number;
    low_price: number;
    close_price: number;
    volume: number;
}

export interface TechnicalIndicators {
    ma5: number;
    ma10: number;
    ma20: number;
    ma50: number;
    ma200: number;
    ema12: number;
    ema26: number;
    macd: number;
    macd_signal: number;
    macd_hist: number;
    rsi: number;
    bb_upper: number;
    bb_middle: number;
    bb_lower: number;
    stoch_k: number;
    stoch_d: number;
    stoch_j: number;
    cci: number;
    sar: number;
    atr: number;
    adx: number;
    obv: number;
    mfi: number;
}

export interface MarketAnalysis {
    symbol: string;
    timestamp: string;
    current_price: number;
    price_change_24h: number;
    price_change_percent: number;
    volume_24h: number;
    trend: string;
    strength: number;
    support_level: number;
    resistance_level: number;
    risk_level: string;
    // 市场状态： "Trending"（趋势）, "Ranging"（震荡）, "Uncertain"（不明朗）
    market_regime?: string;
}

export interface PortfolioRiskMetrics {
    var_95: number;
    var_99: number;
    max_drawdown: number;
    sharpe_ratio: number;
    volatility_ann: number;
    beta: number;
}

// ===== 投资组合类型 =====
export interface PortfolioHolding {
    symbol: string;
    quantity: number;
    average_price: number;
    current_price: number;
    value: number;
    pnl: number;
    pnl_percent: number;
    weight: number;
    risk: string;
}

// ===== 交易信号类型 =====
export interface TradingSignal {
    symbol: string;
    // 时间间隔 (1m, 5m, 15m, 1h, 4h, 1d等)
    interval: string;
    // 毫秒时间戳（后端为 int64 毫秒）
    timestamp: number;
    signal_type: 'buy' | 'sell' | 'hold';
    strength: number;
    price: number;
    stop_loss: number;
    take_profit: number;
    reasoning: string;
    // 0..1 小数（页面显示乘以100为百分比）
    confidence: number;
    risk_level: string;
    // 是否已经发送到 Telegram
    is_sent?: boolean;
}

// ===== 研究报告类型 =====
export interface PriceTargets {
    conservative: number;
    moderate: number;
    aggressive: number;
    time_frame: string;
}


// ===== API请求类型 =====
export interface AnalyzeSymbolRequest {
    symbol: string;
    interval: string;
    limit?: number;
}

// ===== 新闻类型 =====
export interface News {
    id: string;
    title: string;
    content: string;
    link: string;
    source: string;
    sentiment: 'positive' | 'negative' | 'neutral';
    score: number;
    summary: string;
    createdAt: number;
    originalId: string;
}

export interface NewsStatistics {
    total_news: number;
    today_news: number;
    sources: Array<{
        source: string;
        count: number;
    }>;
    sentiment: {
        positive: number;
        negative: number;
        neutral: number;
    };
}