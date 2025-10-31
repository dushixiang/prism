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
                            <a
                                href="https://github.com/dushixiang/prism"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-slate-400 transition-colors hover:text-slate-600"
                                aria-label="GitHub Repository"
                            >
                                <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
                                    <path fillRule="evenodd"
                                          d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
                                          clipRule="evenodd"/>
                                </svg>
                            </a>
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
