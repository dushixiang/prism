import {QueryClient, QueryClientProvider, useQuery} from '@tanstack/react-query';
import {useEffect, useMemo, useRef, useState} from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from './components/ui/sheet';
import {
    type BarData,
    type BusinessDay,
    ColorType,
    createChart,
    type IChartApi,
    type ISeriesApi,
    type LineData,
    LineSeries,
    type MouseEventParams,
    type Time,
    type UTCTimestamp,
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
    holding?: string;
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

type LLMLog = {
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

type LLMLogsResponse = {
    logs: LLMLog[];
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

        // 转换数据格式，并按时间排序、去重
        console.log('Original data points:', data.length);

        const chartDataMap = new Map<number, number>();
        data.forEach((point, index) => {
            const timeSeconds = Math.floor(point.timestamp / 1000);
            if (chartDataMap.has(timeSeconds)) {
                console.warn(`Duplicate timestamp at index ${index}:`, timeSeconds, 'old value:', chartDataMap.get(timeSeconds), 'new value:', point.total_balance);
            }
            // 如果有重复时间戳，保留最新的值
            chartDataMap.set(timeSeconds, point.total_balance);
        });

        // 转换为数组并按时间升序排序
        let chartData = Array.from(chartDataMap.entries())
            .sort(([timeA], [timeB]) => timeA - timeB)
            .map(([time, value]) => ({
                time: time as Time,
                value: value,
            }));

        // 额外的安全检查：过滤掉任何可能的重复时间戳
        const seenTimes = new Set<number>();
        chartData = chartData.filter((point) => {
            const time = point.time as number;
            if (seenTimes.has(time)) {
                console.warn('Duplicate timestamp detected and removed:', time);
                return false;
            }
            seenTimes.add(time);
            return true;
        });

        if (chartData.length === 0) {
            console.error('No valid chart data after filtering');
            return;
        }

        // 最终验证：确保数据严格升序
        for (let i = 1; i < chartData.length; i++) {
            const prevTime = chartData[i - 1].time as number;
            const currTime = chartData[i].time as number;
            if (currTime <= prevTime) {
                console.error('Data not strictly ascending at index', i, 'prev:', prevTime, 'curr:', currTime);
            }
        }

        console.log('Final chart data points:', chartData.length);
        if (chartData.length > 0) {
            console.log('First point:', chartData[0]);
            console.log('Last point:', chartData[chartData.length - 1]);
        }

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

        // 添加初始余额参考线（需要至少2个不同时间点）
        if (initialBalance > 0 && chartData.length >= 2) {
            const minTime = chartData[0].time as number;
            const maxTime = chartData[chartData.length - 1].time as number;

            // 只有当最小时间和最大时间不同时才添加参考线
            if (minTime !== maxTime) {
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

// LLM日志展示组件 - Sheet 抽屉
const LLMLogViewer = ({decisionId}: { decisionId: string }) => {
    const [isOpen, setIsOpen] = useState(false);
    const [selectedRound, setSelectedRound] = useState<number | null>(null);

    const {
        data: logsData,
        isLoading,
        error,
    } = useQuery<LLMLogsResponse>({
        queryKey: ['llm-logs', decisionId],
        queryFn: () => fetcher<LLMLogsResponse>(`/api/trading/llm-logs?decision_id=${decisionId}`),
        enabled: isOpen,
    });

    return (
        <Sheet open={isOpen} onOpenChange={setIsOpen}>
            <SheetTrigger asChild>
                <button
                    className="mt-2 w-full rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 transition hover:bg-blue-100"
                >
                    🔍 查看 LLM 通信日志
                </button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full p-0 sm:max-w-[600px] lg:max-w-[800px]">
                <SheetHeader className="border-b border-slate-200 px-6 py-4">
                    <SheetTitle>LLM 通信日志</SheetTitle>
                    <SheetDescription>
                        {logsData?.logs ? `共 ${logsData.logs.length} 轮对话` : '加载中...'}
                    </SheetDescription>
                </SheetHeader>

                <div className="h-[calc(100vh-80px)] overflow-y-auto px-6 py-4">
                    {isLoading && (
                        <div className="flex h-full items-center justify-center text-slate-500">
                            加载中...
                        </div>
                    )}

                    {error && (
                        <div className="rounded bg-rose-50 p-4 text-sm text-rose-600">
                            {getErrorMessage(error)}
                        </div>
                    )}

                    {logsData?.logs && logsData.logs.length > 0 && (
                        <div className="space-y-3">
                            {logsData.logs.map((log) => (
                                    <div
                                        key={log.id}
                                        className={`rounded-lg border ${
                                            selectedRound === log.round_number
                                                ? 'border-blue-300 bg-blue-50'
                                                : 'border-slate-200 bg-white'
                                        } overflow-hidden transition-all`}
                                    >
                                        {/* 轮次标题 */}
                                        <button
                                            onClick={() =>
                                                setSelectedRound(selectedRound === log.round_number ? null : log.round_number)
                                            }
                                            className="flex w-full items-center justify-between p-3 text-left hover:bg-slate-50"
                                        >
                                            <div className="flex items-center gap-3">
                                                <span className="flex h-6 w-6 items-center justify-center rounded-full bg-blue-500 text-xs font-bold text-white">
                                                    {log.round_number}
                                                </span>
                                                <div className="flex items-center gap-2 text-xs text-slate-600">
                                                    <span className="font-mono">{log.duration}ms</span>
                                                    <span>•</span>
                                                    <span className="font-mono">{log.total_tokens} tokens</span>
                                                </div>
                                            </div>
                                            <svg
                                                className={`h-4 w-4 text-slate-400 transition-transform ${
                                                    selectedRound === log.round_number ? 'rotate-180' : ''
                                                }`}
                                                fill="none"
                                                stroke="currentColor"
                                                viewBox="0 0 24 24"
                                            >
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7"/>
                                            </svg>
                                        </button>

                                        {/* 展开内容 */}
                                        {selectedRound === log.round_number && (
                                            <div className="border-t border-slate-200 p-4">
                                                <div className="space-y-3">
                                                    {(() => {
                                                        // 检查是否有实际内容
                                                        const hasSystemPrompt = log.round_number === 1 && log.system_prompt;
                                                        const hasUserPrompt = log.round_number === 1 && log.user_prompt;
                                                        const hasAssistantContent = log.assistant_content && log.assistant_content.trim();
                                                        const hasToolCalls = log.tool_calls && log.tool_calls !== '[]';
                                                        const hasToolResponses = log.tool_responses && log.tool_responses !== '[]';
                                                        const hasContent = hasSystemPrompt || hasUserPrompt || hasAssistantContent || hasToolCalls || hasToolResponses;

                                                        if (!hasContent && !log.error) {
                                                            return (
                                                                <div className="rounded bg-slate-50 p-4 text-center text-xs text-slate-500">
                                                                    <div className="mb-1">✓ AI 已完成响应</div>
                                                                    <div className="text-slate-400">本轮无额外输出内容</div>
                                                                </div>
                                                            );
                                                        }

                                                        return null;
                                                    })()}

                                                    {/* 系统提示词 */}
                                                    {log.round_number === 1 && log.system_prompt && (
                                                        <div>
                                                            <div className="mb-2 text-xs font-semibold text-slate-700">系统提示词</div>
                                                            <details className="group">
                                                                <summary className="cursor-pointer text-xs text-blue-600 hover:text-blue-800">
                                                                    点击展开查看 ({log.system_prompt.length} 字符)
                                                                </summary>
                                                                <div className="mt-2 max-h-60 overflow-y-auto whitespace-pre-wrap rounded bg-slate-100 p-3 text-xs text-slate-700">
                                                                    {log.system_prompt}
                                                                </div>
                                                            </details>
                                                        </div>
                                                    )}

                                                    {/* 用户提示词 */}
                                                    {log.round_number === 1 && log.user_prompt && (
                                                        <div>
                                                            <div className="mb-2 text-xs font-semibold text-slate-700">用户提示词</div>
                                                            <details className="group">
                                                                <summary className="cursor-pointer text-xs text-blue-600 hover:text-blue-800">
                                                                    点击展开查看 ({log.user_prompt.length} 字符)
                                                                </summary>
                                                                <div className="mt-2 max-h-60 overflow-y-auto whitespace-pre-wrap rounded bg-slate-100 p-3 text-xs text-slate-700">
                                                                    {log.user_prompt}
                                                                </div>
                                                            </details>
                                                        </div>
                                                    )}

                                                    {/* AI 思考 */}
                                                    {log.assistant_content && log.assistant_content.trim() && (
                                                        <div>
                                                            <div className="mb-2 text-xs font-semibold text-slate-700">AI 思考</div>
                                                            <div className="whitespace-pre-wrap rounded bg-blue-50 p-3 text-xs text-slate-700">
                                                                {log.assistant_content}
                                                            </div>
                                                        </div>
                                                    )}

                                                    {/* 工具调用 */}
                                                    {log.tool_calls && log.tool_calls !== '[]' && (
                                                        <div>
                                                            <div className="mb-2 text-xs font-semibold text-slate-700">工具调用</div>
                                                            <div className="space-y-2">
                                                                {(() => {
                                                                    try {
                                                                        const calls = JSON.parse(log.tool_calls);
                                                                        return calls.map((call: any, idx: number) => (
                                                                            <div
                                                                                key={idx}
                                                                                className="rounded bg-amber-50 p-3"
                                                                            >
                                                                                <div className="mb-1 font-semibold text-amber-700">
                                                                                    📞 {call.function}
                                                                                </div>
                                                                                <pre className="overflow-x-auto text-xs text-amber-900">
                                                                                    {JSON.stringify(call.arguments, null, 2)}
                                                                                </pre>
                                                                            </div>
                                                                        ));
                                                                    } catch {
                                                                        return <div className="text-xs text-slate-500">解析失败</div>;
                                                                    }
                                                                })()}
                                                            </div>
                                                        </div>
                                                    )}

                                                    {/* 工具响应 */}
                                                    {log.tool_responses && log.tool_responses !== '[]' && (
                                                        <div>
                                                            <div className="mb-2 text-xs font-semibold text-slate-700">工具响应</div>
                                                            <div className="space-y-2">
                                                                {(() => {
                                                                    try {
                                                                        const responses = JSON.parse(log.tool_responses);
                                                                        return responses.map((response: any, idx: number) => (
                                                                            <div
                                                                                key={idx}
                                                                                className={`rounded p-3 ${
                                                                                    response.error
                                                                                        ? 'bg-rose-50 text-rose-900'
                                                                                        : 'bg-emerald-50 text-emerald-900'
                                                                                }`}
                                                                            >
                                                                                <pre className="overflow-x-auto text-xs">
                                                                                    {JSON.stringify(response.result || response, null, 2)}
                                                                                </pre>
                                                                            </div>
                                                                        ));
                                                                    } catch {
                                                                        return <div className="text-xs text-slate-500">解析失败</div>;
                                                                    }
                                                                })()}
                                                            </div>
                                                        </div>
                                                    )}

                                                    {/* Token统计 */}
                                                    <div className="flex flex-wrap gap-3 border-t border-slate-200 pt-3 text-xs text-slate-600">
                                                        <span>输入: {log.prompt_tokens}</span>
                                                        <span>输出: {log.completion_tokens}</span>
                                                        <span>总计: {log.total_tokens}</span>
                                                        <span>耗时: {log.duration}ms</span>
                                                        {log.finish_reason && <span>结束: {log.finish_reason}</span>}
                                                    </div>

                                                    {/* 错误信息 */}
                                                    {log.error && (
                                                        <div className="rounded bg-rose-50 p-3 text-xs text-rose-700">
                                                            ❌ {log.error}
                                                        </div>
                                                    )}
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        )}

                    {logsData?.logs && logsData.logs.length === 0 && (
                        <div className="flex h-full items-center justify-center text-slate-400">
                            暂无日志记录
                        </div>
                    )}
                </div>
            </SheetContent>
        </Sheet>
    );
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
        refetchInterval: 3000,
    });

    const {
        data: accountData,
    } = useQuery<AccountResponse>({
        queryKey: ['trading-account'],
        queryFn: () => fetcher<AccountResponse>('/api/trading/account'),
        refetchInterval: 10000,
    });

    const {
        data: positionsData,
        isLoading: positionsLoading,
        error: positionsError,
    } = useQuery<PositionsResponse>({
        queryKey: ['trading-positions'],
        queryFn: () => fetcher<PositionsResponse>('/api/trading/positions'),
        refetchInterval: 5000,
    });

    const {
        data: decisionsData,
        error: decisionsError,
    } = useQuery<DecisionsResponse>({
        queryKey: ['trading-decisions'],
        queryFn: () => fetcher<DecisionsResponse>('/api/trading/decisions?limit=10'),
        refetchInterval: 30000,
    });

    const {
        data: tradesData,
        error: tradesError,
    } = useQuery<TradesResponse>({
        queryKey: ['trading-trades'],
        queryFn: () => fetcher<TradesResponse>('/api/trading/trades?limit=100'),
        refetchInterval: 15000,
    });

    const {
        data: equityCurveData,
        error: equityCurveError,
    } = useQuery<EquityCurveResponse>({
        queryKey: ['trading-equity-curve'],
        queryFn: () => fetcher<EquityCurveResponse>('/api/trading/equity-curve'),
        refetchInterval: 30000,
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
                <div
                    className="mx-auto flex h-full max-w-[1920px] flex-col gap-4 px-4 pb-6 pt-4 sm:gap-6 sm:px-6 lg:flex-row">
                    {/* 左侧: 主图表区域 */}
                    <div className={`${cardClass} flex min-h-[320px] flex-1 flex-col p-4 sm:p-6`}>
                        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <h2 className="text-lg font-semibold text-slate-900 sm:text-xl">资金曲线</h2>
                            <div
                                className="flex flex-wrap items-center gap-3 text-xs text-slate-600 sm:gap-4 sm:text-sm">
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
                                                            className="font-mono">{position.holding}</span>
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
                                                        <div
                                                            className="space-y-1 text-[11px] leading-relaxed text-slate-600">
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
                                                <div
                                                    className="prose prose-sm prose-slate max-w-none text-sm [&>*]:mb-2 [&>*:last-child]:mb-0 [&_p]:leading-relaxed [&_ul]:my-2 [&_ol]:my-2 [&_li]:my-1 [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-sm [&_h4]:text-xs [&_strong]:font-semibold [&_code]:bg-slate-100 [&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_code]:text-xs [&_pre]:bg-slate-100 [&_pre]:p-2 [&_pre]:rounded [&_pre]:overflow-x-auto [&_table]:w-full [&_table]:border-collapse [&_table]:my-3 [&_table]:text-xs [&_th]:border [&_th]:border-slate-300 [&_th]:bg-slate-100 [&_th]:px-2 [&_th]:py-1.5 [&_th]:text-left [&_th]:font-semibold [&_td]:border [&_td]:border-slate-300 [&_td]:px-2 [&_td]:py-1.5">
                                                    <ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]}>
                                                        {decision.decision_content ?? ''}
                                                    </ReactMarkdown>
                                                </div>
                                                <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                                                    <span>账户: {formatCurrency(decision.account_value)}</span>
                                                    <span>持仓: {decision.position_count}</span>
                                                    <span>令牌: {decision.prompt_tokens}/{decision.completion_tokens}</span>
                                                </div>

                                                {/* LLM 日志查看器 */}
                                                <LLMLogViewer decisionId={decision.id}/>
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
