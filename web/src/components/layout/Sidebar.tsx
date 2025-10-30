import {useState} from 'react';
import {cardClass} from '@/constants/styles.ts';
import {formatCurrency} from '@/utils/formatters.ts';
import {PositionsList} from '../trading/PositionsList';
import {TradesList} from '../trading/TradesList';
import {DecisionsList} from '../trading/DecisionsList';
import type {Decision, Position, Trade} from '@/types/trading.ts';

interface SidebarProps {
    positions: Position[];
    positionsLoading: boolean;
    positionsError: unknown;
    trades: Trade[] | undefined;
    tradesError: unknown;
    decisions: Decision[] | undefined;
    decisionsError: unknown;
    decisionsCount: number;
    stats: {
        totalTrades: number;
        winningTrades: number;
        losingTrades: number;
        winRate: number;
        totalPnl: number;
    };
}

export const Sidebar = ({
                            positions,
                            positionsLoading,
                            positionsError,
                            trades,
                            tradesError,
                            decisions,
                            decisionsError,
                            decisionsCount,
                            stats,
                        }: SidebarProps) => {
    const [activeTab, setActiveTab] = useState<'positions' | 'trades' | 'decisions'>('positions');

    return (
        <div className={`${cardClass} flex h-full min-h-0 flex-col lg:w-[380px] lg:min-w-[360px]`}>
            {/* 侧边栏标签 */}
            <div className="flex flex-wrap border-b border-slate-200">
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
                    </h3>
                    <span className="text-xs text-slate-500">
                        {activeTab === 'positions' && `共 ${positions.length} 个`}
                        {activeTab === 'trades' && `共 ${stats.totalTrades} 笔`}
                        {activeTab === 'decisions' && `最近 ${decisionsCount} 次`}
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
                        <span className={stats.totalPnl > 0 ? 'text-emerald-600' : stats.totalPnl < 0 ? 'text-rose-600' : 'text-slate-600'}>
                            总盈亏 {formatCurrency(stats.totalPnl)}
                        </span>
                    </div>
                )}
            </div>

            {/* 滚动内容区 */}
            <div className="flex-1 p-4 lg:overflow-y-auto">
                {activeTab === 'positions' && (
                    <PositionsList
                        positions={positions}
                        isLoading={positionsLoading}
                        error={positionsError}
                    />
                )}

                {activeTab === 'trades' && (
                    <TradesList
                        trades={trades}
                        error={tradesError}
                    />
                )}

                {activeTab === 'decisions' && (
                    <DecisionsList
                        decisions={decisions}
                        error={decisionsError}
                    />
                )}
            </div>
        </div>
    );
};
