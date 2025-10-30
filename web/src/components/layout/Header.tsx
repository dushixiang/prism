import {formatCurrency, formatPercent, getPnlColor} from '@/utils/formatters.ts';
import type {AccountMetrics, TradingLoopStatus} from '@/types/trading.ts';

interface HeaderProps {
    loopStatus?: TradingLoopStatus;
    accountMetrics?: AccountMetrics;
}

export const Header = ({loopStatus, accountMetrics}: HeaderProps) => {
    return (
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
                            {loopStatus?.symbols?.slice(0, 3).map((symbol) => (
                                <span key={symbol} className="font-mono text-slate-600">
                                    {symbol}
                                </span>
                            ))}
                        </div>
                    </div>

                    {/* 币种价格展示 */}
                    <div className="hidden flex-wrap gap-4 text-sm text-slate-500 sm:flex">
                        {loopStatus?.symbols?.map((symbol) => (
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
    );
};
