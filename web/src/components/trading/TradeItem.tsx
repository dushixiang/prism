import {formatCurrency, formatDateTime, formatNumber, getPnlColor} from '@/utils/formatters';
import {cardClass} from '@/constants/styles';
import type {Trade} from '@/types/trading';

interface TradeItemProps {
    trade: Trade;
}

export const TradeItem = ({trade}: TradeItemProps) => {
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
                {trade.reason && (
                    <div className="mt-3 border-t border-dashed border-slate-200 pt-3">
                        <div className="text-xs text-slate-500 mb-1">{isClose ? '平仓原因' : '开仓原因'}:</div>
                        <div className="text-xs text-slate-700 leading-relaxed">{trade.reason}</div>
                    </div>
                )}
            </div>
        </div>
    );
};
