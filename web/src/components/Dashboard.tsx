import {useMemo} from 'react';
import {useQuery} from '@tanstack/react-query';
import {fetcher} from '../utils/api';
import {formatCurrency, getErrorMessage} from '../utils/formatters';
import {cardClass} from '../constants/styles';
import {EquityCurveChart} from './charts/EquityCurveChart';
import {Header} from './layout/Header';
import {Sidebar} from './layout/Sidebar';
import type {
    AccountResponse,
    DecisionsResponse,
    EquityCurveResponse,
    PositionsResponse,
    TradesResponse,
    TradingStatusResponse,
} from '../types/trading';

export const Dashboard = () => {
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
            <Header
                loopStatus={statusData?.loop}
                accountMetrics={accountMetrics}
            />

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
                                    <EquityCurveChart
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
                    <Sidebar
                        positions={positions}
                        positionsLoading={positionsLoading}
                        positionsError={positionsError}
                        trades={tradesData?.trades}
                        tradesError={tradesError}
                        decisions={decisionsData?.decisions}
                        decisionsError={decisionsError}
                        decisionsCount={decisionsData?.count ?? 0}
                        stats={stats}
                    />
                </div>
            </div>
        </div>
    );
};
