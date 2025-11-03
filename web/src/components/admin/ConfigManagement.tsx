import {useEffect, useState} from 'react';
import {useQuery, useMutation, useQueryClient} from '@tanstack/react-query';

interface SystemPromptVersion {
    id: string;
    version: number;
    content: string;
    is_active: boolean;
    remark: string;
    created_at: string;
    updated_at: string;
}

interface TradingConfig {
    symbols: string[];
    interval_minutes: number;
    max_drawdown_percent: number;
    max_positions: number;
    max_leverage: number;
    min_leverage: number;
}

interface TradingConfigForm {
    symbols: string;
    interval_minutes: string;
    max_drawdown_percent: string;
    max_positions: string;
    max_leverage: string;
    min_leverage: string;
}

export function ConfigManagement() {
    const [activeTab, setActiveTab] = useState<'system_prompt' | 'trading'>('system_prompt');
    const [editingConfig, setEditingConfig] = useState<string>('');
    const [isEditing, setIsEditing] = useState<boolean>(false);
    const [tradingForm, setTradingForm] = useState<TradingConfigForm | null>(null);
    const [remark, setRemark] = useState<string>('');
    const queryClient = useQueryClient();

    const token = localStorage.getItem('admin_token');

    // 获取系统提示词配置
    const {
        data: systemConfig,
        isLoading: isSystemLoading,
    } = useQuery<SystemPromptVersion>({
        queryKey: ['admin-config', 'system_prompt'],
        queryFn: async () => {
            const response = await fetch('/api/admin/system-prompt', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });
            if (!response.ok) throw new Error('Failed to fetch system prompt');
            return response.json();
        },
    });

    // 获取交易配置
    const {
        data: tradingConfig,
        isLoading: isTradingLoading,
    } = useQuery<TradingConfig>({
        queryKey: ['admin-trading-config'],
        queryFn: async () => {
            const response = await fetch('/api/admin/trading-config', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });
            if (!response.ok) throw new Error('Failed to fetch trading config');
            return response.json();
        },
    });

    // 获取配置历史
    const isSystemTab = activeTab === 'system_prompt';

    const {data: historyData} = useQuery<SystemPromptVersion[]>({
        queryKey: ['admin-system-prompt-history'],
        queryFn: async () => {
            const response = await fetch('/api/admin/system-prompt/history', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });
            if (!response.ok) throw new Error('Failed to fetch history');
            return response.json();
        },
        enabled: isSystemTab,
    });

    // 更新配置
    const updateSystemMutation = useMutation({
        mutationFn: async (data: { content: string; remark: string }) => {
            const response = await fetch('/api/admin/system-prompt', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify(data),
            });
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to update config');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: ['admin-config', 'system_prompt'], exact: true});
            queryClient.invalidateQueries({queryKey: ['admin-system-prompt-history'], exact: true});
            setIsEditing(false);
            setEditingConfig('');
            setRemark('');
        },
    });

    // 回滚配置
    const rollbackMutation = useMutation({
        mutationFn: async (id: string) => {
            const response = await fetch(`/api/admin/system-prompt/history/${id}/rollback`, {
                method: 'GET',
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to rollback');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: ['admin-config', 'system_prompt'], exact: true});
            queryClient.invalidateQueries({queryKey: ['admin-system-prompt-history'], exact: true});
        },
    });

    // 删除配置
    const deleteMutation = useMutation({
        mutationFn: async (id: string) => {
            const response = await fetch(`/api/admin/system-prompt/history/${id}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to delete');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: ['admin-system-prompt-history'], exact: true});
        },
    });

    const updateTradingMutation = useMutation({
        mutationFn: async (config: TradingConfig) => {
            const response = await fetch('/api/admin/trading-config', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify(config),
            });
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to update trading config');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: ['admin-trading-config'], exact: true});
            setIsEditing(false);
            setTradingForm(null);
            setRemark('');
        },
    });

    const handleSave = () => {
        if (isSystemTab) {
            if (!editingConfig.trim()) {
                alert('配置内容不能为空');
                return;
            }

            updateSystemMutation.mutate({
                content: editingConfig,
                remark: remark || '更新配置',
            });
            return;
        }

        if (!tradingForm) {
            alert('交易配置尚未加载，请稍后重试');
            return;
        }

        const symbols = tradingForm.symbols
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean);
        if (symbols.length === 0) {
            alert('请至少填写一个交易币种');
            return;
        }

        const parseNumber = (value: string, fieldLabel: string, allowFloat = false) => {
            if (!value.trim()) {
                throw new Error(`${fieldLabel} 不能为空`);
            }
            const parsed = allowFloat ? Number.parseFloat(value) : Number.parseInt(value, 10);
            if (Number.isNaN(parsed)) {
                throw new Error(`${fieldLabel} 请输入有效的${allowFloat ? '数字' : '整数'}`);
            }
            return parsed;
        };

        let intervalMinutes: number;
        let maxDrawdownPercent: number;
        let maxPositions: number;
        let maxLeverage: number;
        let minLeverage: number;

        try {
            intervalMinutes = parseNumber(tradingForm.interval_minutes, '交易周期', false);
            maxDrawdownPercent = parseNumber(tradingForm.max_drawdown_percent, '最大回撤百分比', true);
            maxPositions = parseNumber(tradingForm.max_positions, '最大持仓数', false);
            maxLeverage = parseNumber(tradingForm.max_leverage, '最大杠杆', false);
            minLeverage = parseNumber(tradingForm.min_leverage, '最小杠杆', false);
        } catch (error) {
            if (error instanceof Error) {
                alert(error.message);
            }
            return;
        }

        updateTradingMutation.mutate({
            symbols,
            interval_minutes: intervalMinutes,
            max_drawdown_percent: maxDrawdownPercent,
            max_positions: maxPositions,
            max_leverage: maxLeverage,
            min_leverage: minLeverage,
        });
    };

    const handleRollback = (id: string, version: number) => {
        if (confirm(`确定要回滚到版本 ${version} 吗？`)) {
            rollbackMutation.mutate(id);
        }
    };

    const handleDelete = (id: string, version: number) => {
        if (confirm(`确定要删除版本 ${version} 吗？此操作不可恢复。`)) {
            deleteMutation.mutate(id);
        }
    };

    const startEditing = () => {
        if (isSystemTab) {
            setEditingConfig(systemConfig?.content ?? '');
        } else {
            if (!tradingConfig) {
                alert('交易配置尚未加载，请稍后重试');
                return;
            }
            setTradingForm({
                symbols: Array.isArray(tradingConfig.symbols) ? tradingConfig.symbols.join(', ') : '',
                interval_minutes: tradingConfig.interval_minutes?.toString() ?? '',
                max_drawdown_percent: tradingConfig.max_drawdown_percent?.toString() ?? '',
                max_positions: tradingConfig.max_positions?.toString() ?? '',
                max_leverage: tradingConfig.max_leverage?.toString() ?? '',
                min_leverage: tradingConfig.min_leverage?.toString() ?? '',
            });
        }
        setRemark('');
        setIsEditing(true);
    };

    const cancelEditing = () => {
        setIsEditing(false);
        setEditingConfig('');
        setTradingForm(null);
        setRemark('');
    };

    useEffect(() => {
        setIsEditing(false);
        setEditingConfig('');
        setTradingForm(null);
        setRemark('');
    }, [activeTab]);

    const isLoading = isSystemTab ? isSystemLoading : isTradingLoading;

    if (isLoading) {
        return <div className="text-center py-8">加载中...</div>;
    }

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-2xl font-bold mb-4">系统配置管理</h2>
                <p className="text-gray-600">管理系统提示词和交易参数配置</p>
            </div>

            {/* Tab 切换 */}
            <div className="border-b">
                <div className="flex space-x-8">
                    <button
                        onClick={() => setActiveTab('system_prompt')}
                        className={`pb-4 px-1 border-b-2 font-medium transition-colors ${
                            activeTab === 'system_prompt'
                                ? 'border-blue-600 text-blue-600'
                                : 'border-transparent text-gray-500 hover:text-gray-700'
                        }`}
                    >
                        系统提示词
                    </button>
                    <button
                        onClick={() => setActiveTab('trading')}
                        className={`pb-4 px-1 border-b-2 font-medium transition-colors ${
                            activeTab === 'trading'
                                ? 'border-blue-600 text-blue-600'
                                : 'border-transparent text-gray-500 hover:text-gray-700'
                        }`}
                    >
                        交易参数
                    </button>
                </div>
            </div>

            {/* 当前配置 */}
            <div className="bg-white rounded-lg shadow p-6">
                {isSystemTab ? (
                    <>
                        <div className="flex justify-between items-start mb-4">
                            <div>
                                <h3 className="text-lg font-semibold">当前配置</h3>
                                <p className="text-sm text-gray-500">
                            版本: {systemConfig?.version || 1} |
                            更新时间: {systemConfig?.created_at ? new Date(systemConfig.created_at).toLocaleString('zh-CN') : '-'}
                                </p>
                            </div>
                            {!isEditing && (
                                <button
                                    onClick={startEditing}
                                    className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700"
                                >
                                    编辑配置
                                </button>
                            )}
                        </div>
                        {isEditing ? (
                            <div className="space-y-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-2">
                                        配置内容
                                    </label>
                                    <textarea
                                        value={editingConfig}
                                        onChange={(e) => setEditingConfig(e.target.value)}
                                        rows={15}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                                        placeholder="输入系统提示词..."
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-2">
                                        修改说明（可选）
                                    </label>
                                    <input
                                        type="text"
                                        value={remark}
                                        onChange={(e) => setRemark(e.target.value)}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        placeholder="简要说明本次修改..."
                                    />
                                </div>
                                <div className="flex space-x-3">
                                    <button
                                        onClick={handleSave}
                                        disabled={updateSystemMutation.isPending}
                                        className="bg-green-600 text-white px-6 py-2 rounded-md hover:bg-green-700 disabled:bg-gray-400"
                                    >
                                        {updateSystemMutation.isPending ? '保存中...' : '保存'}
                                    </button>
                                    <button
                                        onClick={cancelEditing}
                                        disabled={updateSystemMutation.isPending}
                                        className="bg-gray-500 text-white px-6 py-2 rounded-md hover:bg-gray-600"
                                    >
                                        取消
                                    </button>
                                </div>
                                {updateSystemMutation.isError && (
                                    <div className="text-red-600 text-sm">
                                        错误: {updateSystemMutation.error?.message}
                                    </div>
                                )}
                            </div>
                        ) : (
                            <pre className="bg-gray-50 p-4 rounded border overflow-x-auto text-sm whitespace-pre-wrap break-words">
                                {systemConfig?.content || '暂无配置'}
                            </pre>
                        )}
                    </>
                ) : (
                    <>
                        <div className="flex justify-between items-start mb-4">
                            <div>
                                <h3 className="text-lg font-semibold">当前交易参数</h3>
                                <p className="text-sm text-gray-500">
                                    以下为当前生效的交易参数设置。
                                </p>
                            </div>
                            {!isEditing && (
                                <button
                                    onClick={startEditing}
                                    className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700"
                                >
                                    编辑配置
                                </button>
                            )}
                        </div>

                        {isEditing ? (
                            tradingForm && (
                                <div className="space-y-5">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 mb-2">
                                            交易币种（以逗号分隔）
                                        </label>
                                        <input
                                            type="text"
                                            value={tradingForm.symbols}
                                            onChange={(e) =>
                                                setTradingForm({...tradingForm, symbols: e.target.value})
                                            }
                                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            placeholder="例如：BTCUSDT, ETHUSDT"
                                        />
                                    </div>
                                    <div className="grid gap-4 md:grid-cols-2">
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-2">
                                                交易周期（分钟）
                                            </label>
                                            <input
                                                type="number"
                                                min={1}
                                                value={tradingForm.interval_minutes}
                                                onChange={(e) =>
                                                    setTradingForm({...tradingForm, interval_minutes: e.target.value})
                                                }
                                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-2">
                                                最大回撤百分比
                                            </label>
                                            <input
                                                type="number"
                                                step="0.1"
                                                value={tradingForm.max_drawdown_percent}
                                                onChange={(e) =>
                                                    setTradingForm({
                                                        ...tradingForm,
                                                        max_drawdown_percent: e.target.value,
                                                    })
                                                }
                                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-2">
                                                最大持仓数
                                            </label>
                                            <input
                                                type="number"
                                                min={1}
                                                value={tradingForm.max_positions}
                                                onChange={(e) =>
                                                    setTradingForm({...tradingForm, max_positions: e.target.value})
                                                }
                                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-2">
                                                最大杠杆
                                            </label>
                                            <input
                                                type="number"
                                                min={1}
                                                value={tradingForm.max_leverage}
                                                onChange={(e) =>
                                                    setTradingForm({...tradingForm, max_leverage: e.target.value})
                                                }
                                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-2">
                                                最小杠杆
                                            </label>
                                            <input
                                                type="number"
                                                min={1}
                                                value={tradingForm.min_leverage}
                                                onChange={(e) =>
                                                    setTradingForm({...tradingForm, min_leverage: e.target.value})
                                                }
                                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            />
                                        </div>
                                    </div>
                                    <div className="flex space-x-3">
                                        <button
                                            onClick={handleSave}
                                            disabled={updateTradingMutation.isPending}
                                            className="bg-green-600 text-white px-6 py-2 rounded-md hover:bg-green-700 disabled:bg-gray-400"
                                        >
                                            {updateTradingMutation.isPending ? '保存中...' : '保存'}
                                        </button>
                                        <button
                                            onClick={cancelEditing}
                                            disabled={updateTradingMutation.isPending}
                                            className="bg-gray-500 text-white px-6 py-2 rounded-md hover:bg-gray-600"
                                        >
                                            取消
                                        </button>
                                    </div>
                                    {updateTradingMutation.isError && (
                                        <div className="text-red-600 text-sm">
                                            错误: {updateTradingMutation.error?.message}
                                        </div>
                                    )}
                                </div>
                            )
                        ) : (
                            <div className="grid gap-6 md:grid-cols-2">
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">交易币种</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.symbols && tradingConfig.symbols.length > 0
                                            ? tradingConfig.symbols.join(', ')
                                            : '未配置'}
                                    </p>
                                </div>
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">交易周期（分钟）</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.interval_minutes ?? '-'}
                                    </p>
                                </div>
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">最大回撤百分比</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.max_drawdown_percent ?? '-'}
                                    </p>
                                </div>
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">最大持仓数</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.max_positions ?? '-'}
                                    </p>
                                </div>
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">最大杠杆</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.max_leverage ?? '-'}
                                    </p>
                                </div>
                                <div className="space-y-1">
                                    <h4 className="text-sm font-semibold text-gray-700">最小杠杆</h4>
                                    <p className="text-sm text-gray-600">
                                        {tradingConfig?.min_leverage ?? '-'}
                                    </p>
                                </div>
                            </div>
                        )}
                    </>
                )}
            </div>

            {/* 历史版本 */}
            {isSystemTab ? (
                <div className="bg-white rounded-lg shadow p-6">
                    <h3 className="text-lg font-semibold mb-4">历史版本</h3>
                    <div className="space-y-3">
                        {historyData && historyData.length > 0 ? (
                            historyData.map((item) => (
                                <div
                                    key={item.id}
                                    className={`border rounded-lg p-4 ${
                                        item.is_active ? 'border-blue-500 bg-blue-50' : 'border-gray-200'
                                    }`}
                                >
                                    <div className="flex justify-between items-start">
                                        <div className="flex-1">
                                            <div className="flex items-center space-x-2 mb-2">
                                                <span className="font-semibold">版本 {item.version}</span>
                                                {item.is_active && (
                                                    <span className="bg-blue-600 text-white text-xs px-2 py-1 rounded">
                                                        当前版本
                                                    </span>
                                                )}
                                            </div>
                                            <div className="text-sm text-gray-600 space-y-1">
                                                <p>修改说明: {item.remark || '无'}</p>
                                                <p>时间: {new Date(item.created_at).toLocaleString('zh-CN')}</p>
                                            </div>
                                        </div>
                                        {!item.is_active && (
                                            <div className="flex space-x-2">
                                                <button
                                                    onClick={() => handleRollback(item.id, item.version)}
                                                    disabled={rollbackMutation.isPending || deleteMutation.isPending}
                                                    className="bg-yellow-600 text-white px-4 py-2 rounded-md hover:bg-yellow-700 text-sm disabled:bg-gray-400"
                                                >
                                                    回滚到此版本
                                                </button>
                                                <button
                                                    onClick={() => handleDelete(item.id, item.version)}
                                                    disabled={rollbackMutation.isPending || deleteMutation.isPending}
                                                    className="bg-red-600 text-white px-4 py-2 rounded-md hover:bg-red-700 text-sm disabled:bg-gray-400"
                                                >
                                                    删除
                                                </button>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            ))
                        ) : (
                            <p className="text-gray-500 text-center py-4">暂无历史版本</p>
                        )}
                    </div>
                </div>
            ) : (
                <div className="bg-white rounded-lg shadow p-6">
                    <h3 className="text-lg font-semibold mb-2">历史记录说明</h3>
                    <p className="text-sm text-gray-600">
                        交易参数的每一项都会单独记录历史版本，如需查看详细历史，请通过系统日志或数据库查询对应配置项（例如
                        `symbols`、`interval_minutes`）。
                    </p>
                </div>
            )}
        </div>
    );
}
