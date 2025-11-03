import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {fetcher, tradingControlAPI} from '@/utils/api';
import type {TradingStatusResponse} from '@/types/trading';

export function SystemControl() {
    const [loading, setLoading] = useState<'start' | 'stop' | 'restart' | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [success, setSuccess] = useState<string | null>(null);

    const {data: statusData, refetch} = useQuery<TradingStatusResponse>({
        queryKey: ['trading-status'],
        queryFn: () => fetcher<TradingStatusResponse>('/api/trading/status'),
        refetchInterval: 3000,
    });

    const handleControl = async (action: 'start' | 'stop' | 'restart') => {
        setLoading(action);
        setError(null);
        setSuccess(null);
        try {
            await tradingControlAPI[action]();
            const actionText = action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启';
            setSuccess(`系统${actionText}成功`);
            // 等待一小段时间让状态更新
            setTimeout(() => {
                refetch();
            }, 1000);
        } catch (err) {
            setError(err instanceof Error ? err.message : '操作失败');
        } finally {
            setLoading(null);
        }
    };

    const isRunning = statusData?.loop?.is_running ?? false;
    const loopStatus = statusData?.loop;

    return (
        <div className="px-4 py-6 sm:px-0">
            <div className="bg-white rounded-lg shadow">
                <div className="px-6 py-4 border-b border-gray-200">
                    <h2 className="text-xl font-bold text-gray-900">系统控制</h2>
                    <p className="mt-1 text-sm text-gray-600">
                        控制交易系统的启动、停止和重启
                    </p>
                </div>

                <div className="p-6 space-y-6">
                    {/* 系统状态卡片 */}
                    <div className="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-lg p-6 border border-blue-100">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="text-lg font-semibold text-gray-900">系统状态</h3>
                            <div className="flex items-center gap-2">
                                <div className={`h-3 w-3 rounded-full ${isRunning ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'}`}/>
                                <span className={`text-sm font-medium ${isRunning ? 'text-emerald-600' : 'text-slate-600'}`}>
                                    {isRunning ? '运行中' : '已停止'}
                                </span>
                            </div>
                        </div>

                        {loopStatus && (
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                                <div>
                                    <span className="text-gray-600">迭代次数:</span>
                                    <span className="ml-2 font-semibold text-gray-900">{loopStatus.iteration || 0}</span>
                                </div>
                                <div>
                                    <span className="text-gray-600">交易周期:</span>
                                    <span className="ml-2 font-semibold text-gray-900">{loopStatus.interval_minutes || 0} 分钟</span>
                                </div>
                                <div>
                                    <span className="text-gray-600">运行时长:</span>
                                    <span className="ml-2 font-semibold text-gray-900">
                                        {loopStatus.elapsed_hours ? `${loopStatus.elapsed_hours.toFixed(2)} 小时` : 'N/A'}
                                    </span>
                                </div>
                                <div>
                                    <span className="text-gray-600">交易品种:</span>
                                    <span className="ml-2 font-semibold text-gray-900">
                                        {loopStatus.symbols?.join(', ') || 'N/A'}
                                    </span>
                                </div>
                            </div>
                        )}
                    </div>

                    {/* 消息提示 */}
                    {error && (
                        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                            <div className="flex items-center gap-2">
                                <svg className="h-5 w-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                </svg>
                                <span className="text-sm font-medium text-red-800">{error}</span>
                            </div>
                        </div>
                    )}

                    {success && (
                        <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                            <div className="flex items-center gap-2">
                                <svg className="h-5 w-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                </svg>
                                <span className="text-sm font-medium text-green-800">{success}</span>
                            </div>
                        </div>
                    )}

                    {/* 控制按钮 */}
                    <div className="border-t border-gray-200 pt-6">
                        <h3 className="text-lg font-semibold text-gray-900 mb-4">操作面板</h3>
                        <div className="flex flex-wrap gap-3">
                            {!isRunning ? (
                                <button
                                    onClick={() => handleControl('start')}
                                    disabled={loading !== null}
                                    className="flex items-center gap-2 rounded-lg bg-emerald-500 px-6 py-3 text-base font-medium text-white transition-all hover:bg-emerald-600 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                    {loading === 'start' ? (
                                        <>
                                            <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
                                            </svg>
                                            启动中...
                                        </>
                                    ) : (
                                        <>
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"/>
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                            </svg>
                                            启动系统
                                        </>
                                    )}
                                </button>
                            ) : (
                                <>
                                    <button
                                        onClick={() => handleControl('stop')}
                                        disabled={loading !== null}
                                        className="flex items-center gap-2 rounded-lg bg-rose-500 px-6 py-3 text-base font-medium text-white transition-all hover:bg-rose-600 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                        {loading === 'stop' ? (
                                            <>
                                                <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
                                                </svg>
                                                停止中...
                                            </>
                                        ) : (
                                            <>
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z"/>
                                                </svg>
                                                停止系统
                                            </>
                                        )}
                                    </button>
                                    <button
                                        onClick={() => handleControl('restart')}
                                        disabled={loading !== null}
                                        className="flex items-center gap-2 rounded-lg bg-blue-500 px-6 py-3 text-base font-medium text-white transition-all hover:bg-blue-600 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                        {loading === 'restart' ? (
                                            <>
                                                <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
                                                </svg>
                                                重启中...
                                            </>
                                        ) : (
                                            <>
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                                                </svg>
                                                重启系统
                                            </>
                                        )}
                                    </button>
                                </>
                            )}
                        </div>

                        {/* 操作说明 */}
                        <div className="mt-6 bg-blue-50 rounded-lg p-4 border border-blue-100">
                            <h4 className="text-sm font-semibold text-blue-900 mb-2">操作说明</h4>
                            <ul className="text-sm text-blue-800 space-y-1">
                                <li>• <strong>启动系统</strong>: 开始运行交易循环，系统将按配置的时间间隔执行交易决策</li>
                                <li>• <strong>停止系统</strong>: 优雅地停止交易循环，等待当前任务完成后停止</li>
                                <li>• <strong>重启系统</strong>: 停止当前运行并重新启动，应用最新配置</li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
