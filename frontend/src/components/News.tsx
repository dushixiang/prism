import {useQuery} from '@tanstack/react-query';
import {Card} from './ui/card';
import {Button} from './ui/button';
import {Badge} from './ui/badge';
import {LoadingSpinner} from './LoadingSpinner';
import {newsApi} from '../services/api';
import {
    BarChart3,
    Calendar,
    Clock,
    ExternalLink,
    Filter,
    Minus,
    RefreshCw,
    Search,
    TrendingDown,
    TrendingUp
} from 'lucide-react';
import type {News} from '@/types';
import {useSearchParams} from "react-router-dom";

export function NewsPage() {
    const [searchParams, setSearchParams] = useSearchParams({
        limit: '50',
    });

    // 从 searchParams 获取查询参数
    const limit = parseInt(searchParams.get('limit') || '50');
    const keyword = searchParams.get('keyword') || '';
    const source = searchParams.get('source') || '';
    const sentiment = searchParams.get('sentiment') || '';

    // 统一使用 getLatest 接口，通过参数进行筛选
    const {data: newsData, isLoading: newsLoading, refetch: refetchNews} = useQuery({
        queryKey: ['news', limit, keyword, source, sentiment],
        queryFn: () => {
            const params = new URLSearchParams();
            params.set('limit', limit.toString());
            if (keyword) params.set('keyword', keyword);
            if (source) params.set('source', source);
            if (sentiment) params.set('sentiment', sentiment);
            return newsApi.getLatest(params);
        },
    });

    // 获取统计信息
    const {data: statistics, isLoading: statsLoading} = useQuery({
        queryKey: ['news-statistics'],
        queryFn: () => newsApi.getStatistics(),
    });

    const sources = ['金色财经', 'CoinDesk', 'Cointelegraph'];
    const sentiments = [
        {value: 'positive', label: '利好', color: 'text-green-600'},
        {value: 'negative', label: '利空', color: 'text-red-600'},
        {value: 'neutral', label: '中性', color: 'text-gray-600'}
    ];

    // 更新搜索参数的辅助函数
    const updateSearchParams = (updates: Record<string, string | null>) => {
        const newParams = new URLSearchParams(searchParams);
        
        Object.entries(updates).forEach(([key, value]) => {
            if (value === null || value === '') {
                newParams.delete(key);
            } else {
                newParams.set(key, value);
            }
        });
        
        setSearchParams(newParams);
    };

    const handleSearch = () => {
        if (keyword.trim()) {
            // 搜索时清除其他筛选条件
            updateSearchParams({
                keyword: keyword.trim(),
                source: null,
                sentiment: null
            });
        }
    };

    const handleSourceFilter = (selectedSource: string) => {
        updateSearchParams({
            source: selectedSource,
            keyword: null,
            sentiment: null
        });
    };

    const handleSentimentFilter = (selectedSentiment: 'positive' | 'negative' | 'neutral') => {
        updateSearchParams({
            sentiment: selectedSentiment,
            keyword: null,
            source: null
        });
    };

    const handleClearFilters = () => {
        updateSearchParams({
            keyword: null,
            source: null,
            sentiment: null
        });
    };

    const handleRefresh = () => {
        refetchNews();
    };

    // 获取当前新闻列表
    const getCurrentNews = (): News[] => {
        return newsData || [];
    };

    // 获取当前加载状态
    const getCurrentLoading = (): boolean => {
        return newsLoading;
    };

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

    const formatTime = (timestamp: number) => {
        return new Date(timestamp).toLocaleString('zh-CN');
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

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-7xl mx-auto space-y-6">
                {/* 页面标题和控制 */}
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <div
                        className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div>
                            <h1 className="text-3xl font-bold text-gray-900 mb-2">新闻资讯中心</h1>
                            <p className="text-gray-600">实时追踪加密货币市场动态与深度分析</p>
                        </div>

                        <div className="flex items-center space-x-4">
                            <Button onClick={handleRefresh} variant="outline" className="border-black">
                                <RefreshCw className="w-4 h-4 mr-2"/>
                                刷新
                            </Button>
                        </div>
                    </div>

                    {/* 搜索和筛选区域 */}
                    <div className="mt-6 space-y-4">
                        {/* 搜索栏 */}
                        <div className="flex space-x-4">
                            <div className="flex-1 flex items-center space-x-2">
                                <input
                                    type="text"
                                    placeholder="搜索新闻标题或内容..."
                                    value={keyword}
                                    onChange={(e) => {
                                        updateSearchParams({keyword: e.target.value});
                                    }}
                                    onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
                                    className="flex-1 px-4 py-2 border-2 border-gray-300 rounded-lg focus:border-black focus:outline-none"
                                />
                                <Button onClick={handleSearch} className="border-black">
                                    <Search className="w-4 h-4 mr-2"/>
                                    搜索
                                </Button>
                            </div>
                        </div>

                        {/* 筛选按钮 */}
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                            {/* 新闻源筛选 */}
                            <div>
                                <label className="block text-sm font-semibold text-gray-700 mb-2 flex items-center">
                                    <Filter className="w-4 h-4 mr-2"/>
                                    新闻来源
                                </label>
                                <div className="flex flex-wrap gap-2">
                                    <Button
                                        variant={source === '' ? "default" : "outline"}
                                        size="sm"
                                        onClick={handleClearFilters}
                                        className="border-black"
                                    >
                                        全部
                                    </Button>
                                    {sources.map((sourceOption) => (
                                        <Button
                                            key={sourceOption}
                                            variant={source === sourceOption ? "default" : "outline"}
                                            size="sm"
                                            onClick={() => handleSourceFilter(sourceOption)}
                                            className="border-black"
                                        >
                                            {formatSource(sourceOption)}
                                        </Button>
                                    ))}
                                </div>
                            </div>

                            {/* 情绪筛选 */}
                            <div>
                                <label className="block text-sm font-semibold text-gray-700 mb-2 flex items-center">
                                    <BarChart3 className="w-4 h-4 mr-2"/>
                                    市场情绪
                                </label>
                                <div className="flex flex-wrap gap-2">
                                    <Button
                                        variant={sentiment === '' ? "default" : "outline"}
                                        size="sm"
                                        onClick={handleClearFilters}
                                        className="border-black"
                                    >
                                        全部
                                    </Button>
                                    {sentiments.map((sentimentOption) => (
                                        <Button
                                            key={sentimentOption.value}
                                            variant={sentiment === sentimentOption.value ? "default" : "outline"}
                                            size="sm"
                                            onClick={() => handleSentimentFilter(sentimentOption.value as 'positive' | 'negative' | 'neutral')}
                                            className="border-black"
                                        >
                                            {sentimentOption.label}
                                        </Button>
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* 统计信息 */}
                {statistics && !statsLoading && (
                    <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                        <Card className="p-4 bg-white border-2 border-black">
                            <div className="text-center">
                                <p className="text-sm text-gray-600 mb-1">总新闻数</p>
                                <p className="text-2xl font-bold text-gray-900">{statistics.total_news}</p>
                            </div>
                        </Card>
                        <Card className="p-4 bg-white border-2 border-black">
                            <div className="text-center">
                                <p className="text-sm text-gray-600 mb-1">今日新闻</p>
                                <p className="text-2xl font-bold text-blue-600">{statistics.today_news}</p>
                            </div>
                        </Card>
                        <Card className="p-4 bg-white border-2 border-black">
                            <div className="text-center">
                                <p className="text-sm text-gray-600 mb-1">利好</p>
                                <p className="text-2xl font-bold text-green-600">{statistics.sentiment?.positive}</p>
                            </div>
                        </Card>
                        <Card className="p-4 bg-white border-2 border-black">
                            <div className="text-center">
                                <p className="text-sm text-gray-600 mb-1">利空</p>
                                <p className="text-2xl font-bold text-red-600">{statistics.sentiment?.negative}</p>
                            </div>
                        </Card>
                    </div>
                )}

                {/* 新闻列表 */}
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <h2 className="text-xl font-bold text-gray-900 mb-4 flex items-center">
                        <Clock className="w-5 h-5 mr-2"/>
                        {keyword && `搜索结果: "${keyword}"`}
                        {!keyword && source && `来源: ${formatSource(source)}`}
                        {!keyword && !source && sentiment && `${sentiments.find(s => s.value === sentiment)?.label}`}
                        {!keyword && !source && !sentiment && '最新资讯'}
                    </h2>

                    {getCurrentLoading() ? (
                        <div className="text-center py-12">
                            <LoadingSpinner/>
                            <p className="mt-4 text-gray-600">加载新闻中...</p>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {getCurrentNews().map((news) => (
                                <Card key={news.id}
                                      className="p-4 border-2 border-gray-200 hover:border-gray-300 transition-colors">
                                    <div className="flex items-start justify-between">
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-center flex-wrap gap-2 mb-2">
                                                <Badge className={`text-xs px-2 py-1 ${getSentimentColor(news.sentiment)}`}>
                                                    <span className="flex items-center space-x-1">
                                                        {getSentimentIcon(news.sentiment)}
                                                        <span>{sentiments.find(s => s.value === news.sentiment)?.label || news.sentiment}</span>
                                                    </span>
                                                </Badge>
                                                <span className="text-xs text-gray-500">来源：{formatSource(news.source)}</span>
                                                <span className="text-xs text-gray-500 flex items-center">
                                                    <Calendar className="w-3 h-3 mr-1"/>
                                                    {formatTime(news.createdAt)}
                                                </span>
                                                {/* 情绪评分展示 */}
                                                <span className="text-xs text-gray-500">评分：{typeof news.score === 'number' ? news.score.toFixed(2) : 'N/A'}</span>
                                            </div>

                                            <h3 className="text-lg font-semibold text-gray-900 mb-2 line-clamp-2">
                                                {news.title}
                                            </h3>

                                            <p className="text-gray-600 text-sm line-clamp-3 mb-3">
                                                {news.summary}
                                            </p>

                                            {news.link && (
                                                <a
                                                    href={news.link}
                                                    target="_blank"
                                                    rel="noopener noreferrer"
                                                    className="inline-flex items-center text-blue-600 hover:text-blue-800 text-sm"
                                                >
                                                    <ExternalLink className="w-4 h-4 mr-1"/>
                                                    查看原文
                                                </a>
                                            )}
                                        </div>
                                    </div>
                                </Card>
                            ))}

                            {getCurrentNews().length === 0 && !getCurrentLoading() && (
                                <div className="text-center py-12 text-gray-500">
                                    <Clock className="w-16 h-16 mx-auto mb-4 text-gray-300"/>
                                    <p>暂无新闻数据</p>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}