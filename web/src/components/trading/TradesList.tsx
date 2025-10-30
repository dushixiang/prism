import {getErrorMessage} from '@/utils/formatters';
import {TradeItem} from './TradeItem';
import type {Trade} from '@/types/trading';

interface TradesListProps {
    trades: Trade[] | undefined;
    error: unknown;
}

export const TradesList = ({trades, error}: TradesListProps) => {
    if (error) {
        return <p className="text-sm text-rose-500">{getErrorMessage(error)}</p>;
    }

    if (!trades || trades.length === 0) {
        return <p className="text-sm text-slate-500">暂无交易记录</p>;
    }

    return (
        <>
            {trades.map((trade) => (
                <TradeItem key={trade.id} trade={trade}/>
            ))}
        </>
    );
};
