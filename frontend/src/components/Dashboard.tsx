import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {useNavigate} from 'react-router-dom';
import {Card} from './ui/card';
import {Button} from './ui/button';
import {Badge} from './ui/badge';
import {LoadingSpinner} from './LoadingSpinner';
import {marketApi, newsApi} from '../services/api';
import {
    Activity,
    AlertTriangle,
    ArrowDown,
    ArrowUp,
    BarChart3,
    Bitcoin,
    Brain,
    Briefcase,
    Calendar,
    Clock,
    DollarSign,
    Eye,
    ExternalLink,
    Minus,
    RefreshCw,
    Target,
    TrendingDown,
    TrendingUp,
    Zap
} from 'lucide-react';


export function Dashboard() {
    const navigate = useNavigate();
    const [selectedSymbol, setSelectedSymbol] = useState('BTCUSDT');
    const [interval, setInterval] = useState('1h');

    // 获取市场概览（真实数据）
    const {data: marketData, isLoading: marketLoading, refetch: refetchMarket} = useQuery({
        queryKey: ['market-overview'],
        queryFn: () => marketApi.getMarketOverview(),
        refetchInterval: 30000, // 30秒刷新一次
    });

    // 获取热门交易对（5个，包含涨跌幅）
    const {data: trendingData, isLoading: trendingLoading, refetch: refetchTrending} = useQuery({
        queryKey: ['trending-symbols'],
        queryFn: () => marketApi.getTrendingSymbols(5),
        refetchInterval: 60000, // 1分钟刷新一次
    });

    // 获取选中交易对的分析
    const {data: analysisData, isLoading: analysisLoading, refetch: refetchAnalysis} = useQuery({
        queryKey: ['symbol-analysis', selectedSymbol, interval],
        queryFn: () => marketApi.analyzeSymbol({symbol: selectedSymbol, interval}),
        refetchInterval: 30000,
    });

    // 获取最新新闻
    const {data: newsData, isLoading: newsLoading, refetch: refetchNews} = useQuery({
        queryKey: ['latest-news-dashboard'],
        queryFn: () => {
            const params = new URLSearchParams();
            params.set('limit', '5');
            return newsApi.getLatest(params);
        },
        refetchInterval: 300000, // 5分钟刷新一次
    });

    // 获取新闻统计
    const {data: newsStats, isLoading: newsStatsLoading} = useQuery({
        queryKey: ['news-statistics-dashboard'],
        queryFn: () => newsApi.getStatistics(),
        refetchInterval: 300000,
    });

    const overview = marketData;
    const trending = trendingData || [];
    const analysis = analysisData?.analysis;
    const news = newsData || [];

    const handleRefreshAll = () => {
        refetchMarket();
        refetchTrending();
        refetchAnalysis();
        refetchNews();
    };

    // 与 News.tsx 保持一致的渲染工具
    const getSentimentIcon = (s: string) => {
        switch (s) {
            case 'positive':
                return <TrendingUp className="w-4 h-4"/>;
            case 'negative':
                return <TrendingDown className="w-4 h-4"/>;
            default:
                return <Minus className="w-4 h-4"/>;
        }
    };

    const getSentimentColor = (s: string) => {
        switch (s) {
            case 'positive':
                return 'text-green-600 bg-green-50 border-green-200';
            case 'negative':
                return 'text-red-600 bg-red-50 border-red-200';
            default:
                return 'text-gray-600 bg-gray-50 border-gray-200';
        }
    };

    const formatSource = (src: string) => {
        switch (src) {
            case 'jinse':
                return '金色财经';
            case 'coindesk':
                return 'CoinDesk';
            case 'cointelegraph':
                return 'Cointelegraph';
            default:
                return src;
        }
    };

    const formatTime = (timestamp: number) => new Date(timestamp).toLocaleString('zh-CN');
    const regimeLabel = (r?: string) => r === 'Trending' ? '趋势' : r === 'Ranging' ? '震荡' : '不确定';
    const regimeBadgeClass = (r?: string) => r === 'Trending'
        ? 'bg-green-50 text-green-600 border-green-200'
        : r === 'Ranging'
            ? 'bg-blue-50 text-blue-600 border-blue-200'
            : 'bg-gray-50 text-gray-600 border-gray-200';

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-7xl mx-auto space-y-6">
                {/* 页面标题 */}
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <div
                        className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div>
                            <h1 className="text-3xl font-bold text-gray-900 mb-2 flex items-center">
                                <BarChart3 className="w-8 h-8 mr-3"/>
                                市场分析仪表盘
                            </h1>
                            <p className="text-gray-600">实时加密货币市场数据与智能分析</p>
                        </div>
                        <Button
                            onClick={handleRefreshAll}
                            variant="outline"
                            className="border-black"
                        >
                            <RefreshCw className="w-4 h-4 mr-2"/>
                            刷新数据
                        </Button>
                    </div>
                </div>

                {/* 市场概览卡片 */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    {marketLoading ? (
                        <div className="col-span-4 flex justify-center py-12">
                            <LoadingSpinner/>
                        </div>
                    ) : overview ? (
                        <>
                            <Card className="p-6 bg-white border-2 border-black">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <p className="text-sm font-medium text-gray-600 mb-1">总市值</p>
                                        <p className="text-2xl font-bold text-gray-900">{overview.total_market_cap}</p>
                                    </div>
                                    <div className="p-3 bg-blue-50 rounded-lg">
                                        <BarChart3 className="w-6 h-6 text-blue-600"/>
                                    </div>
                                </div>
                            </Card>

                            <Card className="p-6 bg-white border-2 border-black">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <p className="text-sm font-medium text-gray-600 mb-1">24h成交量</p>
                                        <p className="text-2xl font-bold text-gray-900">{overview.total_volume_24h}</p>
                                    </div>
                                    <div className="p-3 bg-green-50 rounded-lg">
                                        <DollarSign className="w-6 h-6 text-green-600"/>
                                    </div>
                                </div>
                            </Card>

                            <Card className="p-6 bg-white border-2 border-black">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <p className="text-sm font-medium text-gray-600 mb-1">BTC占比</p>
                                        <p className="text-2xl font-bold text-gray-900">{overview.btc_dominance}</p>
                                    </div>
                                    <div className="p-3 bg-orange-50 rounded-lg">
                                        <Bitcoin className="w-6 h-6 text-orange-600"/>
                                    </div>
                                </div>
                            </Card>

                            <Card className="p-6 bg-white border-2 border-black">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <p className="text-sm font-medium text-gray-600 mb-1">恐贪指数</p>
                                        <p className={`text-2xl font-bold ${
                                            (overview.fear_greed_index || 0) > 70 ? 'text-red-600' :
                                                (overview.fear_greed_index || 0) < 30 ? 'text-green-600' : 'text-yellow-600'
                                        }`}>{overview.fear_greed_index}</p>
                                    </div>
                                    <div className="p-3 bg-purple-50 rounded-lg">
                                        <Activity className="w-6 h-6 text-purple-600"/>
                                    </div>
                                </div>
                            </Card>
                        </>
                    ) : (
                        <div className="col-span-4 text-center text-gray-500 py-12">
                            <AlertTriangle className="w-16 h-16 mx-auto mb-4 text-gray-300"/>
                            <p>无法获取市场概览数据</p>
                        </div>
                    )}
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                    {/* 热门交易对 */}
                    <Card className="p-6 bg-white border-2 border-black">
                        <h3 className="text-lg font-semibold mb-4 flex items-center">
                            <TrendingUp className="w-5 h-5 mr-2"/>
                            热门交易对
                        </h3>
                        {trendingLoading ? (
                            <div className="flex justify-center py-8">
                                <LoadingSpinner/>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {trending.map((item: any) => (
                                    <div
                                        key={item.symbol}
                                        className={`flex items-center justify-between p-3 rounded-lg cursor-pointer transition-colors border-2 ${
                                            selectedSymbol === item.symbol
                                                ? 'bg-blue-50 border-blue-200'
                                                : 'hover:bg-gray-50 border-gray-200'
                                        }`}
                                        onClick={() => setSelectedSymbol(item.symbol)}
                                    >
                                        <div className="flex flex-col">
                                            <span className="font-medium text-gray-900">{item.symbol}</span>
                                            <span className="text-sm text-gray-500">${parseFloat(item.price).toFixed(2)}</span>
                                        </div>
                                        <div className="flex items-center space-x-2">
                                            <Badge
                                                className={`${
                                                    parseFloat(item.percent_change) >= 0
                                                        ? 'bg-green-50 text-green-600 border-green-200'
                                                        : 'bg-red-50 text-red-600 border-red-200'
                                                }`}
                                            >
                                                {parseFloat(item.percent_change) >= 0 ? <ArrowUp className="w-3 h-3 mr-1"/> :
                                                    <ArrowDown className="w-3 h-3 mr-1"/>}
                                                {parseFloat(item.percent_change).toFixed(2)}%
                                            </Badge>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </Card>

                    {/* 当前分析 */}
                    <Card className="p-6 bg-white border-2 border-black lg:col-span-2">
                        <div
                            className="flex flex-col space-y-4 md:flex-row md:items-center md:justify-between md:space-y-0 mb-6">
                            <h3 className="text-lg font-semibold flex items-center">
                                <Target className="w-5 h-5 mr-2"/>
                                {selectedSymbol} 技术分析
                            </h3>
                            <div className="flex space-x-2">
                                {['1h', '4h', '1d', '1w'].map((timeframe) => (
                                    <Button
                                        key={timeframe}
                                        variant={interval === timeframe ? "default" : "outline"}
                                        size="sm"
                                        onClick={() => setInterval(timeframe)}
                                        className="border-black"
                                    >
                                        {timeframe}
                                    </Button>
                                ))}
                            </div>
                        </div>

                        {analysisLoading ? (
                            <div className="flex justify-center py-12">
                                <LoadingSpinner/>
                            </div>
                        ) : analysis ? (
                            <div className="space-y-6">
                                {/* 价格信息 */}
                                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                                    <div className="text-center p-4 bg-gray-50 rounded-lg border-2 border-gray-200">
                                        <p className="text-sm font-medium text-gray-600 mb-1">当前价格</p>
                                        <p className="text-xl font-bold text-gray-900">
                                            ${analysis.current_price?.toFixed(4)}
                                        </p>
                                    </div>
                                    <div className="text-center p-4 bg-gray-50 rounded-lg border-2 border-gray-200">
                                        <p className="text-sm font-medium text-gray-600 mb-1">24h变化</p>
                                        <p className={`text-xl font-bold flex items-center justify-center ${
                                            analysis.price_change_percent >= 0 ? 'text-green-600' : 'text-red-600'
                                        }`}>
                                            {analysis.price_change_percent >= 0 ? <ArrowUp className="w-4 h-4 mr-1"/> :
                                                <ArrowDown className="w-4 h-4 mr-1"/>}
                                            {analysis.price_change_percent >= 0 ? '+' : ''}
                                            {analysis.price_change_percent?.toFixed(2)}%
                                        </p>
                                    </div>
                                    <div className="text-center p-4 bg-gray-50 rounded-lg border-2 border-gray-200">
                                        <p className="text-sm font-medium text-gray-600 mb-1">趋势强度</p>
                                        <p className="text-xl font-bold text-gray-900">{analysis.strength?.toFixed(1)}/10</p>
                                    </div>
                                    <div className="text-center p-4 bg-gray-50 rounded-lg border-2 border-gray-200">
                                        <p className="text-sm font-medium text-gray-600 mb-1">风险等级</p>
                                        <Badge
                                            className={`${
                                                analysis.risk_level === 'low' ? 'bg-green-50 text-green-600 border-green-200' :
                                                    analysis.risk_level === 'medium' ? 'bg-yellow-50 text-yellow-600 border-yellow-200' :
                                                        'bg-red-50 text-red-600 border-red-200'
                                            }`}
                                        >
                                            {analysis.risk_level}
                                        </Badge>
                                    </div>
                                </div>

                                {/* 技术指标 */}
                                <div>
                                    <h4 className="font-semibold mb-3 flex items-center">
                                        <BarChart3 className="w-4 h-4 mr-2"/>
                                        关键技术指标
                                    </h4>
                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">RSI</p>
                                            <p className="font-semibold">{analysisData.technical_indicators?.rsi?.toFixed(2)}</p>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">MACD</p>
                                            <p className="font-semibold">{analysisData.technical_indicators?.macd?.toFixed(4)}</p>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">MA20</p>
                                            <p className="font-semibold">${analysisData.technical_indicators?.ma20?.toFixed(2)}</p>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">支撑位</p>
                                            <p className="font-semibold">${analysis.support_level?.toFixed(2)}</p>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">阻力位</p>
                                            <p className="font-semibold">${analysis.resistance_level?.toFixed(2)}</p>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">趋势</p>
                                            <Badge className="bg-blue-50 text-blue-600 border-blue-200">
                                                {analysis.trend === 'up' ? '上涨' :
                                                    analysis.trend === 'down' ? '下跌' : '震荡'}
                                            </Badge>
                                        </div>
                                        <div className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200">
                                            <p className="text-xs text-gray-500 mb-1">市场状态</p>
                                            <Badge className={`${regimeBadgeClass(analysis.market_regime)} border-2`}>
                                                {regimeLabel(analysis.market_regime)}
                                            </Badge>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        ) : (
                            <div className="text-center text-gray-500 py-12">
                                <Target className="w-16 h-16 mx-auto mb-4 text-gray-300"/>
                                <p>选择交易对查看技术分析</p>
                            </div>
                        )}
                    </Card>
                </div>

                {/* 新闻动态和快速操作 */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                    {/* 最新新闻 */}
                    <Card className="p-6 bg-white border-2 border-black">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="text-lg font-semibold flex items-center">
                                <Clock className="w-5 h-5 mr-2"/>
                                最新资讯
                            </h3>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => navigate('/news')}
                                className="border-black"
                            >
                                <Eye className="w-4 h-4 mr-2"/>
                                查看更多
                            </Button>
                        </div>

                        {newsLoading ? (
                            <div className="flex justify-center py-8">
                                <LoadingSpinner/>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {news.map((item) => (
                                    <div key={item.id}
                                         className="p-3 bg-gray-50 rounded-lg border-2 border-gray-200 hover:border-gray-300 transition-colors">
                                        <div className="flex items-center flex-wrap gap-2 mb-2">
                                            <Badge className={`text-xs px-2 py-1 ${getSentimentColor(item.sentiment)}`}>
                                                <span className="flex items-center space-x-1">
                                                    {getSentimentIcon(item.sentiment)}
                                                    <span>{item.sentiment === 'positive' ? '利好' : item.sentiment === 'negative' ? '利空' : '中性'}</span>
                                                </span>
                                            </Badge>
                                            <span className="text-xs text-gray-500">来源：{formatSource(item.source)}</span>
                                            <span className="text-xs text-gray-500 flex items-center">
                                                <Calendar className="w-3 h-3 mr-1"/>
                                                {formatTime(item.createdAt)}
                                            </span>
                                            <span className="text-xs text-gray-500">评分：{typeof item.score === 'number' ? item.score.toFixed(2) : 'N/A'}</span>
                                        </div>

                                        <h4 className="font-medium text-sm text-gray-900 line-clamp-2 mb-2">
                                            {item.summary}
                                        </h4>

                                        {item.link && (
                                            <a
                                                href={item.link}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="inline-flex items-center text-blue-600 hover:text-blue-800 text-xs"
                                            >
                                                <ExternalLink className="w-4 h-4 mr-1"/>
                                                查看原文
                                            </a>
                                        )}
                                    </div>
                                ))}

                                {news.length === 0 && (
                                    <div className="text-center text-gray-500 py-8">
                                        <Clock className="w-12 h-12 mx-auto mb-2 text-gray-300"/>
                                        <p>暂无新闻数据</p>
                                    </div>
                                )}
                            </div>
                        )}
                    </Card>

                    {/* 快速操作 */}
                    <Card className="p-6 bg-white border-2 border-black">
                        <h3 className="text-lg font-semibold mb-4 flex items-center">
                            <Zap className="w-5 h-5 mr-2"/>
                            快速操作
                        </h3>
                        <div className="grid grid-cols-2 gap-4">
                            <Button
                                variant="outline"
                                className="h-20 flex flex-col items-center justify-center space-y-2 border-black hover:bg-gray-50"
                                onClick={() => navigate('/market-analysis')}
                            >
                                <BarChart3 className="w-6 h-6 text-blue-600"/>
                                <span className="text-sm font-medium">市场分析</span>
                            </Button>

                            <Button
                                variant="outline"
                                className="h-20 flex flex-col items-center justify-center space-y-2 border-black hover:bg-gray-50"
                                onClick={() => navigate('/prediction')}
                            >
                                <Brain className="w-6 h-6 text-purple-600"/>
                                <span className="text-sm font-medium">智能预测</span>
                            </Button>

                            <Button
                                variant="outline"
                                className="h-20 flex flex-col items-center justify-center space-y-2 border-black hover:bg-gray-50"
                                onClick={() => navigate('/portfolio')}
                            >
                                <Briefcase className="w-6 h-6 text-green-600"/>
                                <span className="text-sm font-medium">投资组合</span>
                            </Button>

                            <Button
                                variant="outline"
                                className="h-20 flex flex-col items-center justify-center space-y-2 border-black hover:bg-gray-50"
                                onClick={() => navigate('/signals')}
                            >
                                <Zap className="w-6 h-6 text-orange-600"/>
                                <span className="text-sm font-medium">交易信号</span>
                            </Button>
                        </div>

                        {/* 新闻统计 */}
                        {newsStats && !newsStatsLoading && (
                            <div className="mt-4 pt-4 border-t-2 border-gray-200">
                                <h4 className="text-sm font-medium text-gray-700 mb-2">今日统计</h4>
                                <div className="grid grid-cols-3 gap-2 text-xs">
                                    <div className="text-center p-2 bg-blue-50 rounded border-2 border-blue-200">
                                        <p className="font-semibold text-blue-600">{newsStats.today_news}</p>
                                        <p className="text-gray-600">新闻</p>
                                    </div>
                                    <div className="text-center p-2 bg-green-50 rounded border-2 border-green-200">
                                        <p className="font-semibold text-green-600">{newsStats.sentiment?.positive}</p>
                                        <p className="text-gray-600">积极</p>
                                    </div>
                                    <div className="text-center p-2 bg-red-50 rounded border-2 border-red-200">
                                        <p className="font-semibold text-red-600">{newsStats.sentiment?.negative}</p>
                                        <p className="text-gray-600">消极</p>
                                    </div>
                                </div>
                            </div>
                        )}
                    </Card>
                </div>
            </div>
        </div>
    );
}