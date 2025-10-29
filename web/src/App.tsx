import {QueryClient, QueryClientProvider, useQuery} from '@tanstack/react-query';
import {useEffect, useMemo, useRef, useState} from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {
    ColorType,
    createChart,
    type IChartApi,
    type ISeriesApi,
    LineSeries,
    type Time,
    type BusinessDay,
    type UTCTimestamp,
    type MouseEventParams,
    type LineData,
    type BarData,
} from 'lightweight-charts';

type TradingLoopStatus = {
    is_running: boolean;
    iteration: number;
    start_time: string;
    elapsed_hours: number;
    symbols: string[];
    interval_minutes: number;
};

type AccountMetrics = {
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

type Position = {
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
    holding_hours?: number;
    holding_cycles?: number;
    remaining_hours?: number;
    opened_at?: string;
    warnings?: string[];
    entry_reason?: string;
    exit_plan?: string;
};

type Decision = {
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

type Trade = {
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

type TradingStatusResponse = {
    loop: TradingLoopStatus;
    account?: AccountMetrics;
    positions?: Position[];
};

type AccountResponse = AccountMetrics;

type PositionsResponse = {
    count: number;
    positions: Position[];
};

type DecisionsResponse = {
    count: number;
    decisions: Decision[];
};

type TradesResponse = {
    count: number;
    trades: Trade[];
};

type EquityCurveDataPoint = {
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

type EquityCurveResponse = {
    count: number;
    data: EquityCurveDataPoint[];
};

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            retry: 1,
            refetchOnWindowFocus: false,
            staleTime: 15000,
        },
    },
});

const fetcher = async <T, >(url: string): Promise<T> => {
    const response = await fetch(url);
    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || '请求失败');
    }
    return response.json() as Promise<T>;
};

const formatCurrency = (value: number | undefined) => {
    if (value === undefined || Number.isNaN(value)) {
        return '-';
    }
    return value.toLocaleString('zh-CN', {style: 'currency', currency: 'USD'});
};

const formatPercent = (value: number | undefined) => {
    if (value === undefined || Number.isNaN(value)) {
        return '-';
    }
    const sign = value > 0 ? '+' : '';
    return `${sign}${value.toFixed(2)}%`;
};

const formatNumber = (value: number | undefined, fractionDigits = 2) => {
    if (value === undefined || Number.isNaN(value)) {
        return '-';
    }
    return value.toLocaleString('zh-CN', {
        minimumFractionDigits: fractionDigits,
        maximumFractionDigits: fractionDigits,
    });
};

const formatDateTime = (value?: string) => {
    if (!value) {
        return '-';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return value;
    }
    return date.toLocaleString('zh-CN', {
        timeZone: 'Asia/Shanghai',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
    });
};

const getErrorMessage = (error: unknown) => {
    if (!error) {
        return '';
    }
    return error instanceof Error ? error.message : '未知错误';
};

const getPnlColor = (value: number | undefined) => {
    if (value === undefined || Number.isNaN(value)) {
        return 'text-slate-600';
    }
    if (value > 0) {
        return 'text-emerald-600';
    }
    if (value < 0) {
        return 'text-rose-600';
    }
    return 'text-slate-600';
};

const cardClass = 'rounded-md border border-slate-200 bg-white shadow-sm';

// 主资金曲线图表组件
const MainEquityCurveChart = ({data, initialBalance}: { data: EquityCurveDataPoint[]; initialBalance: number }) => {
    const chartContainerRef = useRef<HTMLDivElement>(null);
    const chartRef = useRef<IChartApi | null>(null);
    const seriesRef = useRef<ISeriesApi<any> | null>(null);

    const formatTimestampToCST = (epochSeconds: number) => {
        if (!Number.isFinite(epochSeconds)) {
            return '-';
        }
        return new Date(epochSeconds * 1000).toLocaleString('zh-CN', {
            timeZone: 'Asia/Shanghai',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false,
        });
    };

    const toEpochSeconds = (time: Time): number | null => {
        if (typeof time === 'number') {
            return time;
        }
        if (typeof time === 'string') {
            const parsed = Number(time);
            return Number.isFinite(parsed) ? parsed : null;
        }
        const businessDay = time as BusinessDay;
        if (typeof businessDay?.year === 'number' && typeof businessDay?.month === 'number' && typeof businessDay?.day === 'number') {
            return Math.floor(Date.UTC(businessDay.year, businessDay.month - 1, businessDay.day) / 1000);
        }
        return null;
    };

    useEffect(() => {
        if (!chartContainerRef.current || data.length === 0) return;

        chartContainerRef.current.style.position = 'relative';

        // 创建图表
        const chart = createChart(chartContainerRef.current, {
            layout: {
                background: {type: ColorType.Solid, color: '#ffffff'},
                textColor: '#64748b',
            },
            width: chartContainerRef.current.clientWidth,
            height: chartContainerRef.current.clientHeight,
            grid: {
                vertLines: {color: '#f1f5f9', style: 1},
                horzLines: {color: '#f1f5f9', style: 1},
            },
            rightPriceScale: {
                borderColor: '#e2e8f0',
                textColor: '#64748b',
            },
            timeScale: {
                borderColor: '#e2e8f0',
                timeVisible: true,
                secondsVisible: false,
            },
            localization: {
                timeFormatter: (time: UTCTimestamp | BusinessDay) => {
                    const seconds = toEpochSeconds(time as Time);
                    return seconds ? formatTimestampToCST(seconds) : '';
                },
            },
            crosshair: {
                mode: 1,
                vertLine: {
                    color: '#cbd5e1',
                    width: 1,
                    style: 3,
                    labelBackgroundColor: '#2862E3',
                },
                horzLine: {
                    color: '#cbd5e1',
                    width: 1,
                    style: 3,
                    labelBackgroundColor: '#2862E3',
                },
            },
        });

        chartRef.current = chart;

        // 创建线系列
        const lineSeries = chart.addSeries(LineSeries, {
            color: '#2862E3',
            lineWidth: 3,
            priceFormat: {
                type: 'price',
                precision: 2,
                minMove: 0.01,
            },
            lastValueVisible: true,
            priceLineVisible: true,
        });

        seriesRef.current = lineSeries;

        // 转换数据格式
        const chartData = data.map((point) => ({
            time: (point.timestamp / 1000) as Time,
            value: point.total_balance,
        }));

        lineSeries.setData(chartData);

        const tooltip = document.createElement('div');
        tooltip.style.position = 'absolute';
        tooltip.style.display = 'none';
        tooltip.style.pointerEvents = 'none';
        tooltip.style.zIndex = '50';
        tooltip.style.padding = '6px 10px';
        tooltip.style.borderRadius = '8px';
        tooltip.style.background = 'rgba(30, 41, 59, 0.92)';
        tooltip.style.color = '#f8fafc';
        tooltip.style.fontSize = '12px';
        tooltip.style.lineHeight = '1.4';
        tooltip.style.border = '1px solid rgba(148, 163, 184, 0.35)';
        tooltip.style.boxShadow = '0 12px 32px rgba(15, 23, 42, 0.25)';
        tooltip.style.whiteSpace = 'nowrap';

        chartContainerRef.current.appendChild(tooltip);

        const handleCrosshairMove = (param: MouseEventParams<Time>) => {
            const container = chartContainerRef.current;
            const point = param.point;
            const timeValue = param.time;

            if (!container || !point || timeValue === undefined) {
                tooltip.style.display = 'none';
                return;
            }

            const x = Number(point.x);
            const y = Number(point.y);
            if (!Number.isFinite(x) || !Number.isFinite(y) || x < 0 || y < 0 ||
                x > container.clientWidth || y > container.clientHeight) {
                tooltip.style.display = 'none';
                return;
            }

            const seriesValue = param.seriesData.get(lineSeries);
            let price: number | undefined;
            if (seriesValue) {
                const typedSeries = seriesValue as Partial<LineData<Time>> & Partial<BarData<Time>>;
                if (typeof typedSeries.value === 'number') {
                    price = typedSeries.value;
                } else if (typeof typedSeries.close === 'number') {
                    price = typedSeries.close;
                }
            }

            if (price === undefined) {
                tooltip.style.display = 'none';
                return;
            }

            const epochSeconds = toEpochSeconds(timeValue as Time);
            if (!epochSeconds) {
                tooltip.style.display = 'none';
                return;
            }

            tooltip.innerHTML = `
                <div style="font-size:11px;color:#cbd5f5;margin-bottom:2px;">${formatTimestampToCST(epochSeconds)}</div>
                <div style="font-size:12px;color:#f8fafc;font-weight:600;">余额：${formatCurrency(price)}</div>
            `;

            tooltip.style.display = 'block';
            const tooltipRect = tooltip.getBoundingClientRect();
            const containerWidth = container.clientWidth;
            const containerHeight = container.clientHeight;

            let left = x;
            let top = y - 12;

            if (left < tooltipRect.width / 2 + 8) {
                left = tooltipRect.width / 2 + 8;
            } else if (left > containerWidth - tooltipRect.width / 2 - 8) {
                left = containerWidth - tooltipRect.width / 2 - 8;
            }

            if (top < tooltipRect.height + 12) {
                top = y + tooltipRect.height + 12;
            }
            if (top > containerHeight - 12) {
                top = containerHeight - 12;
            }

            tooltip.style.left = `${left}px`;
            tooltip.style.top = `${top}px`;
        };

        chart.subscribeCrosshairMove(handleCrosshairMove);

        // 添加初始余额参考线
        if (initialBalance > 0) {
            const minTime = Math.min(...data.map(d => d.timestamp / 1000));
            const maxTime = Math.max(...data.map(d => d.timestamp / 1000));

            const referenceLine = chart.addSeries(LineSeries, {
                color: '#94a3b8',
                lineWidth: 1,
                lineStyle: 3,
                priceFormat: {
                    type: 'price',
                    precision: 2,
                    minMove: 0.01,
                },
                lastValueVisible: false,
                priceLineVisible: false,
            });

            referenceLine.setData([
                {time: minTime as Time, value: initialBalance},
                {time: maxTime as Time, value: initialBalance},
            ]);
        }

        // 自适应内容
        chart.timeScale().fitContent();

        // 响应式调整
        const handleResize = () => {
            if (chartContainerRef.current) {
                chart.applyOptions({
                    width: chartContainerRef.current.clientWidth,
                    height: chartContainerRef.current.clientHeight,
                });
            }
        };

        window.addEventListener('resize', handleResize);

        return () => {
            window.removeEventListener('resize', handleResize);
            chart.unsubscribeCrosshairMove(handleCrosshairMove);
            if (tooltip.parentNode) {
                tooltip.parentNode.removeChild(tooltip);
            }
            chart.remove();
        };
    }, [data, initialBalance]);

    if (data.length === 0) {
        return (
            <div className="flex h-full items-center justify-center text-slate-400">
                暂无资金曲线数据
            </div>
        );
    }

    return <div ref={chartContainerRef} className="relative h-full w-full"/>;
};

// 交易列表项组件
const TradeItem = ({trade}: { trade: Trade }) => {
    const isLong = trade.side.toLowerCase() === 'long' || trade.side.toLowerCase() === 'buy';
    const isClose = trade.type.toLowerCase() === 'close';
    const notional = trade.price * trade.quantity;

    return (
        <div className={`${cardClass} mb-3 p-3 sm:p-4`}>
            <div className="mb-3 flex items-center justify-between text-xs text-slate-500">
                <span className="flex items-center gap-3">
                    <span className={`text-sm font-semibold ${isLong ? 'text-emerald-600' : 'text-rose-600'}`}>
                        {isLong ? '做多' : '做空'}
                    </span>
                    <span className="font-mono text-sm font-semibold text-slate-900">{trade.symbol}</span>
                    {isClose && <span className="text-slate-500">已平仓</span>}
                </span>
                <span className="font-mono text-xs text-slate-400">{formatDateTime(trade.executed_at)}</span>
            </div>

            <div className="space-y-1 text-xs text-slate-700">
                <div className="flex justify-between">
                    <span className="text-slate-500">价格:</span>
                    <span className="font-mono text-slate-900">${formatNumber(trade.price, 4)}</span>
                </div>
                <div className="flex justify-between">
                    <span className="text-slate-500">数量:</span>
                    <span className="font-mono text-slate-900">{formatNumber(trade.quantity, 4)}</span>
                </div>
                <div className="flex justify-between">
                    <span className="text-slate-500">名义价值:</span>
                    <span className="font-mono text-slate-900">${formatNumber(notional, 0)}</span>
                </div>
                {trade.leverage > 1 && (
                    <div className="flex justify-between">
                        <span className="text-slate-500">杠杆:</span>
                        <span className="font-mono text-slate-900">{trade.leverage}x</span>
                    </div>
                )}
                {isClose && trade.pnl !== 0 && (
                    <div
                        className="mt-3 flex items-center justify-between border-t border-dashed border-slate-200 pt-3 text-xs">
                        <span className="text-slate-500">净盈亏:</span>
                        <span className={`font-mono font-semibold ${getPnlColor(trade.pnl)}`}>
                            {formatCurrency(trade.pnl)}
                        </span>
                    </div>
                )}
            </div>
        </div>
    );
};

const Dashboard = () => {
    const [activeTab, setActiveTab] = useState<'all' | 'positions' | 'trades' | 'decisions'>('all');

    const {
        data: statusData,
    } = useQuery<TradingStatusResponse>({
        queryKey: ['trading-status'],
        queryFn: () => fetcher<TradingStatusResponse>('/api/trading/status'),
        refetchInterval: 15000,
    });

    const {
        data: accountData,
    } = useQuery<AccountResponse>({
        queryKey: ['trading-account'],
        queryFn: () => fetcher<AccountResponse>('/api/trading/account'),
        refetchInterval: 30000,
    });

    const {
        data: positionsData,
        isLoading: positionsLoading,
        error: positionsError,
    } = useQuery<PositionsResponse>({
        queryKey: ['trading-positions'],
        queryFn: () => fetcher<PositionsResponse>('/api/trading/positions'),
        refetchInterval: 20000,
    });

    const {
        data: decisionsData,
        error: decisionsError,
    } = useQuery<DecisionsResponse>({
        queryKey: ['trading-decisions'],
        queryFn: () => fetcher<DecisionsResponse>('/api/trading/decisions?limit=10'),
        refetchInterval: 60000,
    });

    const {
        data: tradesData,
        error: tradesError,
    } = useQuery<TradesResponse>({
        queryKey: ['trading-trades'],
        queryFn: () => fetcher<TradesResponse>('/api/trading/trades?limit=100'),
        refetchInterval: 30000,
    });

    const {
        data: equityCurveData,
        error: equityCurveError,
    } = useQuery<EquityCurveResponse>({
        queryKey: ['trading-equity-curve'],
        queryFn: () => fetcher<EquityCurveResponse>('/api/trading/equity-curve'),
        refetchInterval: 60000,
    });

    const accountMetrics = accountData ?? statusData?.account;
    const positions = useMemo(
        () => positionsData?.positions ?? statusData?.positions ?? [],
        [positionsData?.positions, statusData?.positions],
    );

    // 计算统计数据
    const stats = useMemo(() => {
        const trades = tradesData?.trades ?? [];
        const closedTrades = trades.filter(t => t.type.toLowerCase() === 'close');
        const winningTrades = closedTrades.filter(t => t.pnl > 0);
        const losingTrades = closedTrades.filter(t => t.pnl < 0);
        const winRate = closedTrades.length > 0 ? (winningTrades.length / closedTrades.length) * 100 : 0;
        const totalPnl = closedTrades.reduce((sum, t) => sum + t.pnl, 0);

        return {
            totalTrades: trades.length,
            winningTrades: winningTrades.length,
            losingTrades: losingTrades.length,
            winRate,
            totalPnl,
        };
    }, [tradesData?.trades]);

    return (
        <div className="flex min-h-screen flex-col bg-slate-50 lg:h-screen lg:overflow-hidden">
            {/* 顶部导航栏 */}
            <header className="shrink-0 border-b border-slate-200 bg-white/95 backdrop-blur">
                <div
                    className="mx-auto flex max-w-[1920px] flex-col gap-6 px-4 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8 lg:py-5">
                    <div className="flex flex-col gap-4">
                        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                            <div className="flex items-center gap-3">
                                <h1 className="text-xl font-semibold text-slate-900 sm:text-2xl">Prism 交易监控</h1>
                                <span className="text-xs text-slate-500 sm:text-sm">策略状态一目了然</span>
                            </div>
                            <div className="flex items-center gap-2 text-xs text-slate-500 sm:hidden">
                                {statusData?.loop.symbols?.slice(0, 3).map((symbol) => (
                                    <span key={symbol} className="font-mono text-slate-600">
                                        {symbol}
                                    </span>
                                ))}
                            </div>
                        </div>

                        {/* 币种价格展示 */}
                        <div className="hidden flex-wrap gap-4 text-sm text-slate-500 sm:flex">
                            {statusData?.loop.symbols?.map((symbol) => (
                                <span key={symbol} className="font-mono text-slate-600">
                                    {symbol}
                                </span>
                            ))}
                        </div>
                    </div>

                    {/* 账户统计 */}
                    <div className="flex flex-wrap items-center gap-4 text-xs text-slate-600 sm:gap-6 sm:text-sm">
                        {accountMetrics && (
                            <>
                                <div className="flex flex-col gap-1 text-right">
                                    <span className="text-xs uppercase tracking-[0.2em] text-slate-400">总资产</span>
                                    <span className="font-mono text-base font-semibold text-slate-900 sm:text-lg">
                                        {formatCurrency(accountMetrics.total_balance)}
                                    </span>
                                </div>
                                <div className="flex flex-col gap-1 text-right">
                                    <span className="text-xs uppercase tracking-[0.2em] text-slate-400">收益率</span>
                                    <span
                                        className={`font-mono text-base font-semibold sm:text-lg ${getPnlColor(accountMetrics.return_percent)}`}>
                                        {formatPercent(accountMetrics.return_percent)}
                                    </span>
                                </div>
                                <div className="flex flex-col gap-1 text-right">
                                    <span className="text-xs uppercase tracking-[0.2em] text-slate-400">最大回撤</span>
                                    <span className="font-mono text-base font-semibold text-rose-600 sm:text-lg">
                                        {formatPercent(accountMetrics.drawdown_from_peak)}
                                    </span>
                                </div>
                            </>
                        )}
                    </div>
                </div>
            </header>

            {/* 主内容区 */}
            <div className="flex-1 overflow-hidden">
                <div className="mx-auto flex h-full max-w-[1920px] flex-col gap-4 px-4 pb-6 pt-4 sm:gap-6 sm:px-6 lg:flex-row">
                    {/* 左侧: 主图表区域 */}
                    <div className={`${cardClass} flex min-h-[320px] flex-1 flex-col p-4 sm:p-6`}>
                        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <h2 className="text-lg font-semibold text-slate-900 sm:text-xl">资金曲线</h2>
                            <div className="flex flex-wrap items-center gap-3 text-xs text-slate-600 sm:gap-4 sm:text-sm">
                                <span>
                                    初始: {formatCurrency(accountMetrics?.initial_balance)}
                                </span>
                                <span>
                                    峰值: {formatCurrency(accountMetrics?.peak_balance)}
                                </span>
                                <span className="font-semibold text-slate-900">
                                    当前: {formatCurrency(accountMetrics?.total_balance)}
                                </span>
                            </div>
                        </div>
                        <div className="flex-1 overflow-hidden">
                            {equityCurveError ? (
                                <div className="flex h-full items-center justify-center text-rose-500">
                                    {getErrorMessage(equityCurveError)}
                                </div>
                            ) : equityCurveData?.data ? (
                                <div className="h-[260px] sm:h-[360px] lg:h-full">
                                    <MainEquityCurveChart
                                        data={equityCurveData.data}
                                        initialBalance={accountMetrics?.initial_balance ?? 10000}
                                    />
                                </div>
                            ) : (
                                <div className="flex h-full items-center justify-center text-slate-400">
                                    加载中...
                                </div>
                            )}
                        </div>
                    </div>

                    {/* 右侧: 信息面板 */}
                    <div className={`${cardClass} flex h-full min-h-0 flex-col lg:w-[380px] lg:min-w-[360px]`}>
                        {/* 侧边栏标签 */}
                        <div className="flex flex-wrap border-b border-slate-200">
                            <button
                                onClick={() => setActiveTab('all')}
                                className={`flex-1 border-r border-slate-200 px-3 py-2 text-xs font-medium transition sm:px-4 sm:py-3 sm:text-sm ${
                                    activeTab === 'all'
                                        ? 'bg-blue-50 text-blue-700'
                                        : 'text-slate-600 hover:bg-slate-50'
                                }`}
                            >
                                全部
                            </button>
                            <button
                                onClick={() => setActiveTab('positions')}
                                className={`flex-1 border-r border-slate-200 px-3 py-2 text-xs font-medium transition sm:px-4 sm:py-3 sm:text-sm ${
                                    activeTab === 'positions'
                                        ? 'bg-blue-50 text-blue-700'
                                        : 'text-slate-600 hover:bg-slate-50'
                                }`}
                            >
                                持仓 ({positions.length})
                            </button>
                            <button
                                onClick={() => setActiveTab('trades')}
                                className={`flex-1 border-r border-slate-200 px-3 py-2 text-xs font-medium transition sm:px-4 sm:py-3 sm:text-sm ${
                                    activeTab === 'trades'
                                        ? 'bg-blue-50 text-blue-700'
                                        : 'text-slate-600 hover:bg-slate-50'
                                }`}
                            >
                                交易 ({stats.totalTrades})
                            </button>
                            <button
                                onClick={() => setActiveTab('decisions')}
                                className={`flex-1 px-3 py-2 text-xs font-medium transition sm:px-4 sm:py-3 sm:text-sm ${
                                    activeTab === 'decisions'
                                        ? 'bg-blue-50 text-blue-700'
                                        : 'text-slate-600 hover:bg-slate-50'
                                }`}
                            >
                                决策
                            </button>
                        </div>

                        {/* 内容头部 */}
                        <div className="border-b border-slate-200 p-4">
                            <div className="mb-2 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                                <h3 className="text-sm font-semibold text-slate-900 sm:text-base">
                                    {activeTab === 'positions' && '当前持仓'}
                                    {activeTab === 'trades' && '交易历史'}
                                    {activeTab === 'decisions' && 'AI决策记录'}
                                    {activeTab === 'all' && '最近交易'}
                                </h3>
                                <span className="text-xs text-slate-500">
                                        {activeTab === 'positions' && `共 ${positions.length} 个`}
                                    {activeTab === 'trades' && `共 ${stats.totalTrades} 笔`}
                                    {activeTab === 'decisions' && `最近 ${decisionsData?.count ?? 0} 次`}
                                    {activeTab === 'all' && '最近 100 笔'}
                                    </span>
                            </div>
                            {activeTab === 'trades' && stats.totalTrades > 0 && (
                                <div className="mt-2 flex flex-wrap gap-3 text-xs sm:text-sm">
                                        <span className="text-emerald-600">
                                            胜 {stats.winningTrades}
                                        </span>
                                    <span className="text-rose-600">
                                            负 {stats.losingTrades}
                                        </span>
                                    <span className="text-slate-600">
                                            胜率 {stats.winRate.toFixed(1)}%
                                        </span>
                                    <span className={getPnlColor(stats.totalPnl)}>
                                            总盈亏 {formatCurrency(stats.totalPnl)}
                                        </span>
                                </div>
                            )}
                        </div>

                        {/* 滚动内容区 */}
                        <div className="flex-1 p-4 lg:overflow-y-auto">
                            {/* 持仓列表 */}
                            {activeTab === 'positions' && (
                                <>
                                    {positionsError && (
                                        <p className="text-sm text-rose-500">{getErrorMessage(positionsError)}</p>
                                    )}
                                    {positionsLoading && (
                                        <p className="text-sm text-slate-500">加载中...</p>
                                    )}
                                    {positions.length === 0 && !positionsLoading && (
                                        <p className="text-sm text-slate-500">当前无持仓</p>
                                    )}
                                    {positions.map((position) => {
                                        const isLong = position.side.toLowerCase() === 'long' || position.side.toLowerCase() === 'buy';
                                        return (
                                            <div
                                                key={position.id}
                                                className={`${cardClass} mb-3 p-3 sm:p-4`}
                                            >
                                                <div className="mb-2 flex items-center justify-between">
                                                        <span className="flex items-center gap-3">
                                                            <span
                                                                className={`text-sm font-semibold ${isLong ? 'text-emerald-600' : 'text-rose-600'}`}>
                                                                {isLong ? '做多' : '做空'}
                                                            </span>
                                                            <span
                                                                className="font-mono text-sm font-semibold text-slate-900">
                                                                {position.symbol}
                                                            </span>
                                                        </span>
                                                    {position.leverage && (
                                                        <span
                                                            className="text-xs text-slate-600">{position.leverage}x</span>
                                                    )}
                                                </div>

                                                <div className="space-y-1 text-xs text-slate-700">
                                                    <div className="flex justify-between">
                                                        <span className="text-slate-500">开仓价:</span>
                                                        <span
                                                            className="font-mono">${formatNumber(position.entry_price, 4)}</span>
                                                    </div>
                                                    <div className="flex justify-between">
                                                        <span className="text-slate-500">现价:</span>
                                                        <span
                                                            className="font-mono">${formatNumber(position.current_price, 4)}</span>
                                                    </div>
                                                    <div className="flex justify-between">
                                                        <span className="text-slate-500">数量:</span>
                                                        <span
                                                            className="font-mono">{formatNumber(position.quantity, 4)}</span>
                                                    </div>
                                                    <div className="flex justify-between">
                                                        <span className="text-slate-500">持仓时间:</span>
                                                        <span
                                                            className="font-mono">{formatNumber(position.holding_hours, 1)}小时</span>
                                                    </div>
                                                    <div
                                                        className="flex justify-between border-t border-slate-200 pt-1">
                                                        <span className="text-slate-500">未实现盈亏:</span>
                                                        <span
                                                            className={`font-mono font-semibold ${getPnlColor(position.pnl_percent)}`}>
                                                                {formatPercent(position.pnl_percent)} ({formatCurrency(position.unrealized_pnl)})
                                                            </span>
                                                    </div>
                                                    <div className="pt-2 text-slate-600">
                                                        <div className="mb-1 font-medium">策略信息</div>
                                                        <div className="space-y-1 text-[11px] leading-relaxed text-slate-600">
                                                            <div>
                                                                <span className="text-slate-500">开仓理由：</span>
                                                                <span>{position.entry_reason?.trim() || '未提供'}</span>
                                                            </div>
                                                            <div>
                                                                <span className="text-slate-500">退出计划：</span>
                                                                <span>{position.exit_plan?.trim() || '未提供'}</span>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    {position.warnings && position.warnings.length > 0 && (
                                                        <div
                                                            className="mt-2 rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-700">
                                                            {position.warnings.map((w) => (
                                                                <div key={w}>⚠️ {w}</div>
                                                            ))}
                                                        </div>
                                                    )}
                                                </div>
                                            </div>
                                        );
                                    })}
                                </>
                            )}

                            {/* 交易历史列表 */}
                            {(activeTab === 'trades' || activeTab === 'all') && (
                                <>
                                    {tradesError && (
                                        <p className="text-sm text-rose-500">{getErrorMessage(tradesError)}</p>
                                    )}
                                    {tradesData?.trades && tradesData.trades.length > 0 ? (
                                        tradesData.trades.map((trade) => (
                                            <TradeItem key={trade.id} trade={trade}/>
                                        ))
                                    ) : (
                                        <p className="text-sm text-slate-500">暂无交易记录</p>
                                    )}
                                </>
                            )}

                            {/* AI决策列表 */}
                            {activeTab === 'decisions' && (
                                <>
                                    {decisionsError && (
                                        <p className="text-sm text-rose-500">{getErrorMessage(decisionsError)}</p>
                                    )}
                                    {decisionsData?.decisions && decisionsData.decisions.length > 0 ? (
                                        decisionsData.decisions.map((decision) => (
                                            <div
                                                key={decision.id}
                                                className={`${cardClass} mb-3 p-3 sm:p-4`}
                                            >
                                                <div
                                                    className="mb-2 flex items-center justify-between text-xs text-slate-600">
                                                    <span>第 {decision.iteration} 次迭代</span>
                                                    <span>{formatDateTime(decision.executed_at)}</span>
                                                </div>
                                                <div className="prose prose-sm prose-slate max-w-none text-sm">
                                                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                                        {decision.decision_content ?? ''}
                                                    </ReactMarkdown>
                                                </div>
                                                <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                                                    <span>账户: {formatCurrency(decision.account_value)}</span>
                                                    <span>持仓: {decision.position_count}</span>
                                                    <span>令牌: {decision.prompt_tokens}/{decision.completion_tokens}</span>
                                                </div>
                                            </div>
                                        ))
                                    ) : (
                                        <p className="text-sm text-slate-500">暂无决策记录</p>
                                    )}
                                </>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <Dashboard/>
        </QueryClientProvider>
    );
}

export default App;
