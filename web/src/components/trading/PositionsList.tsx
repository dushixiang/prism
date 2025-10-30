import {formatCurrency, formatNumber, formatPercent, getErrorMessage, getPnlColor} from '@/utils/formatters';
import {cardClass} from '@/constants/styles';
import type {Position} from '@/types/trading';

interface PositionsListProps {
    positions: Position[];
    isLoading: boolean;
    error: unknown;
}

export const PositionsList = ({positions, isLoading, error}: PositionsListProps) => {
    if (error) {
        return <p className="text-sm text-rose-500">{getErrorMessage(error)}</p>;
    }

    if (isLoading) {
        return <p className="text-sm text-slate-500">加载中...</p>;
    }

    if (positions.length === 0) {
        return <p className="text-sm text-slate-500">当前无持仓</p>;
    }

    return (
        <>
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
    );
};
