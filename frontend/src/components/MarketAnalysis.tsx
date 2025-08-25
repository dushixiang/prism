import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Card} from './ui/card';
import {Button} from './ui/button';
import {LoadingSpinner} from './LoadingSpinner';
import {KlineChart} from './KlineChart';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from './ui/select';
import {marketApi} from '../services/api';
import {Badge} from './ui/badge';
import {AlertTriangle, BarChart2, BarChart3, Clock, Crosshair, Minus, RefreshCw, Target, TrendingDown, TrendingUp} from 'lucide-react';

export function MarketAnalysis() {
    const [selectedSymbol, setSelectedSymbol] = useState('BTCUSDT');
    const [interval, setInterval] = useState('1h');

    // 交易对列表
    const {data: symbolsResp} = useQuery({
        queryKey: ['market-symbols'],
        queryFn: () => marketApi.getSymbols(),
        staleTime: 10 * 60 * 1000,
    });
    const symbols: string[] = symbolsResp?.symbols || [];

    // 技术分析查询
    const {data: technicalData, isLoading: technicalLoading, refetch: refetchTechnical} = useQuery({
        queryKey: ['technical-analysis', selectedSymbol, interval],
        queryFn: () => marketApi.analyzeSymbol({symbol: selectedSymbol, interval}),
    });

    // K线数据查询
    const {data: klineData, isLoading: klineLoading} = useQuery({
        queryKey: ['kline-data', selectedSymbol, interval],
        queryFn: () => marketApi.getKlineData(selectedSymbol, interval, 200),
    });

    const indicators = technicalData?.technical_indicators;

    const popularSymbols = ['BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'ADAUSDT', 'SOLUSDT', 'XRPUSDT', 'DOTUSDT', 'AVAXUSDT', 'LINKUSDT', 'MATICUSDT'];
    const timeframes = ['1m', '3m', '5m', '15m', '30m', '1h', '2h', '4h', '6h', '8h', '12h', '1d', '3d', '1w', '1M'];

    const handleRefresh = () => {
        refetchTechnical();
    };

    const trendLabel = (t?: string) => t === 'up' ? '上涨' : t === 'down' ? '下跌' : '震荡';
    const trendIcon = (t?: string) => t === 'up' ? (
        <TrendingUp className="w-5 h-5 text-green-600 mr-1"/>
    ) : t === 'down' ? (
        <TrendingDown className="w-5 h-5 text-red-600 mr-1"/>
    ) : (
        <Minus className="w-5 h-5 text-gray-600 mr-1"/>
    );
    const regimeLabel = (r?: string) => r === 'Trending' ? '趋势' : r === 'Ranging' ? '震荡' : '不确定';
    const regimeBadgeClass = (r?: string) => r === 'Trending'
        ? 'bg-green-50 text-green-600 border-green-200'
        : r === 'Ranging'
            ? 'bg-blue-50 text-blue-600 border-blue-200'
            : 'bg-gray-50 text-gray-600 border-gray-200';
    const riskBadgeClass = (risk?: string) => risk === 'low'
        ? 'bg-green-50 text-green-600 border-green-200'
        : risk === 'medium'
            ? 'bg-yellow-50 text-yellow-600 border-yellow-200'
            : 'bg-red-50 text-red-600 border-red-200';

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-7xl mx-auto space-y-6">
                {/* 页面标题和控制 */}
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div>
                            <h1 className="text-3xl font-bold text-gray-900 mb-2">市场技术分析</h1>
                            <p className="text-gray-600">专业的加密货币技术分析与关键指标面板</p>
                        </div>
                        <div className="flex items-center space-x-4">
                            <Button onClick={handleRefresh} variant="outline" className="border-black">
                                <RefreshCw className="w-4 h-4 mr-2"/>
                                刷新
                            </Button>
                        </div>
                    </div>

                    {/* 参数区 */}
                    <div className="mt-6 space-y-6">
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                            {/* 交易对选择 */}
                            <div>
                                <label className="block text-sm font-semibold text-gray-700 mb-3 flex items-center">
                                    <Target className="w-4 h-4 mr-2"/>
                                    选择交易对
                                </label>
                                <div className="grid grid-cols-2 gap-2 mb-3">
                                    {popularSymbols.map((symbol) => (
                                        <Button key={symbol} variant={selectedSymbol === symbol ? 'default' : 'outline'} size="sm" onClick={() => setSelectedSymbol(symbol)} className="h-10 font-medium border-black">{symbol}</Button>
                                    ))}
                                </div>
                                <Select value={selectedSymbol} onValueChange={setSelectedSymbol}>
                                    <SelectTrigger className="w-full border-2 border-black bg-white">
                                        <SelectValue placeholder="选择交易对"/>
                                    </SelectTrigger>
                                    <SelectContent>
                                        {(symbols.length === 0 ? [selectedSymbol] : symbols).map(s => (
                                            <SelectItem key={s} value={s}>{s}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            {/* 时间周期选择 */}
                            <div>
                                <label className="block text-sm font-semibold text-gray-700 mb-3 flex items-center">
                                    <Clock className="w-4 h-4 mr-2"/>
                                    时间周期
                                </label>
                                <div className="grid grid-cols-7 gap-2">
                                    {timeframes.map((tf) => (
                                        <Button key={tf} variant={interval === tf ? 'default' : 'outline'} size="sm" onClick={() => setInterval(tf)} className="h-10 font-medium border-black">{tf}</Button>
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* 技术分析图表 */}
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    {klineLoading ? (
                        <div className="text-center py-16">
                            <LoadingSpinner/>
                            <p className="mt-4 text-gray-600">加载K线数据中...</p>
                        </div>
                    ) : klineData?.kline_data ? (
                        <KlineChart data={klineData.kline_data} indicators={indicators} symbol={selectedSymbol} height={600}/>
                    ) : (
                        <div className="text-center text-gray-500 py-16">
                            <BarChart2 className="w-16 h-16 mx-auto mb-4 text-gray-300"/>
                            <p>暂无K线数据</p>
                        </div>
                    )}
                </div>

                {/* 技术分析结果区 */}
                <div className="space-y-6">
                    {technicalLoading ? (
                        <Card className="p-8 text-center bg-white border-2 border-black">
                            <LoadingSpinner/>
                            <p className="mt-4 text-gray-600">正在分析 {selectedSymbol}...</p>
                        </Card>
                    ) : (
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                            {/* 核心指标 */}
                            {indicators && (
                                <Card className="p-6 bg-white border-2 border-black">
                                    <h3 className="text-lg font-bold mb-4 text-gray-900 flex items-center">
                                        <BarChart3 className="w-5 h-5 mr-2"/>
                                        核心指标
                                    </h3>
                                    <div className="space-y-4">
                                        <div className="grid grid-cols-2 gap-4">
                                            <div className="text-center">
                                                <p className="text-xs text-gray-600 mb-1">RSI</p>
                                                <p className={`text-lg font-bold ${indicators.rsi > 70 ? 'text-red-600' : indicators.rsi < 30 ? 'text-green-600' : 'text-gray-800'}`}>{indicators.rsi?.toFixed(1) || 'N/A'}</p>
                                            </div>
                                            <div className="text-center">
                                                <p className="text-xs text-gray-600 mb-1">MACD</p>
                                                <p className={`text-lg font-bold ${indicators.macd > 0 ? 'text-green-600' : 'text-red-600'}`}>{indicators.macd?.toFixed(4) || 'N/A'}</p>
                                            </div>
                                            <div className="text-center">
                                                <p className="text-xs text-gray-600 mb-1">KDJ-K</p>
                                                <p className="text-lg font-bold text-gray-800">{indicators.stoch_k?.toFixed(1) || 'N/A'}</p>
                                            </div>
                                            <div className="text-center">
                                                <p className="text-xs text-gray-600 mb-1">ATR</p>
                                                <p className="text-lg font-bold text-gray-800">{indicators.atr?.toFixed(4) || 'N/A'}</p>
                                            </div>
                                        </div>
                                    </div>
                                </Card>
                            )}
                            {/* 趋势分析 */}
                            {technicalData?.analysis && (
                                <Card className="p-6 bg-white border-2 border-black">
                                    <h3 className="text-lg font-bold mb-4 text-gray-900 flex items-center">
                                        <TrendingUp className="w-5 h-5 mr-2"/>
                                        趋势分析
                                    </h3>
                                    <div className="space-y-4">
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">趋势方向</p>
                                            <div className="flex items-center justify-center mb-1">
                                                {trendIcon(technicalData.analysis.trend)}
                                            </div>
                                            <p className={`text-lg font-bold ${technicalData.analysis.trend === 'up' ? 'text-green-600' : technicalData.analysis.trend === 'down' ? 'text-red-600' : 'text-gray-600'}`}>{trendLabel(technicalData.analysis.trend) || 'N/A'}</p>
                                        </div>
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">趋势强度</p>
                                            <p className="text-xl font-bold text-blue-600">{technicalData.analysis.strength?.toFixed(1) || 'N/A'}/10</p>
                                        </div>
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">风险评级</p>
                                            <Badge className={`${riskBadgeClass(technicalData.analysis.risk_level)} border-2`}>{technicalData.analysis.risk_level || 'N/A'}</Badge>
                                        </div>
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">市场状态</p>
                                            <Badge className={`${regimeBadgeClass(technicalData.analysis.market_regime)} border-2`}>{regimeLabel(technicalData.analysis.market_regime)}</Badge>
                                        </div>
                                    </div>
                                </Card>
                            )}
                            {/* 关键价位 */}
                            {technicalData?.analysis && (
                                <Card className="p-6 bg-white border-2 border-black">
                                    <h3 className="text-lg font-bold mb-4 text-gray-900 flex items-center">
                                        <Crosshair className="w-5 h-5 mr-2"/>
                                        关键价位
                                    </h3>
                                    <div className="space-y-4">
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">支撑位</p>
                                            <p className="text-lg font-bold text-green-600">${technicalData.analysis.support_level?.toFixed(4) || 'N/A'}</p>
                                        </div>
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">阻力位</p>
                                            <p className="text-lg font-bold text-red-600">${technicalData.analysis.resistance_level?.toFixed(4) || 'N/A'}</p>
                                        </div>
                                        <div className="text-center">
                                            <p className="text-xs text-gray-600 mb-1">24h涨跌</p>
                                            <p className={`text-lg font-bold ${technicalData.analysis.price_change_percent > 0 ? 'text-green-600' : 'text-red-600'}`}>{technicalData.analysis.price_change_percent?.toFixed(2) || 'N/A'}%</p>
                                        </div>
                                    </div>
                                </Card>
                            )}
                        </div>
                    )}
                </div>

                {/* 风险提示 */}
                <Card className="p-6 bg-white border-2 border-black">
                    <h4 className="font-bold text-gray-800 mb-4 flex items-center">
                        <AlertTriangle className="w-5 h-5 mr-2"/>
                        风险提示
                    </h4>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-sm text-gray-700">
                        <div className="flex items-center space-x-2"><BarChart3 className="w-4 h-4"/><span>技术分析仅供参考</span></div>
                        <div className="flex items-center space-x-2"><Clock className="w-4 h-4"/><span>市场波动剧烈，注意风险</span></div>
                        <div className="flex items-center space-x-2"><Crosshair className="w-4 h-4"/><span>请合理控制仓位</span></div>
                        <div className="flex items-center space-x-2"><Minus className="w-4 h-4"/><span>投资需谨慎，盈亏自负</span></div>
                    </div>
                </Card>
            </div>
        </div>
    );
}