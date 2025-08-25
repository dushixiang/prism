import {useEffect, useRef, useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Card} from '../ui/card';
import {Button} from '../ui/button';
import {LoadingSpinner} from '../LoadingSpinner';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '../ui/select';
import {marketApi, newsApi} from '@/services/api.ts';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {AlertTriangle, Bot, Target} from 'lucide-react';
import {tokenManager} from '@/utils/token';

export function AIAnalysis() {
    const [selectedSymbol, setSelectedSymbol] = useState('BTCUSDT');
    const [promptText, setPromptText] = useState('');
    const [selectedNewsIds, setSelectedNewsIds] = useState<string[]>([]);

    const [content, setContent] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const eventRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null);

    const {data: symbolsResp} = useQuery({
        queryKey: ['market-symbols'],
        queryFn: () => marketApi.getSymbols(),
        staleTime: 10 * 60 * 1000,
    });
    const symbols: string[] = symbolsResp?.symbols || [];

    const {data: latestNews} = useQuery({
        queryKey: ['latest-news-for-llm-page'],
        queryFn: () => {
            const params = new URLSearchParams();
            params.set('limit', '50');
            return newsApi.getLatest(params);
        },
        staleTime: 60 * 1000,
    });

    const buildPrompt = async () => {
        setError(null);
        try {
            const res = await marketApi.buildPrompt({
                symbol: selectedSymbol,
                interval: '1h',
                limit: 200,
                news_ids: selectedNewsIds
            });
            setPromptText(res.prompt || '');
        } catch (e: any) {
            setPromptText('');
            setError(e?.message || '生成提示词失败');
        }
    };

    const startStream = async () => {
        setError(null);
        setContent('');
        setLoading(true);
        try {
            const resp = await fetch('/api/market/llm/stream', {
                method: 'POST',
                headers: {'Content-Type': 'application/json', 'Prism-Token': tokenManager.getToken() || ''},
                body: JSON.stringify({prompt: promptText}),
            });
            if (!resp.ok || !resp.body) throw new Error('连接失败');
            const reader = resp.body.getReader();
            eventRef.current = reader;
            const decoder = new TextDecoder();
            let buffer = '';
            while (true) {
                const {value, done} = await reader.read();
                if (done) break;
                buffer += decoder.decode(value, {stream: true});
                const parts = buffer.split('\n\n');
                buffer = parts.pop() || '';
                for (const chunk of parts) {
                    const line = chunk.trim();
                    if (!line.startsWith('data:')) continue;
                    const payload = line.slice(5).trim();
                    try {
                        const data = JSON.parse(payload);
                        if (data.type === 'content') setContent(prev => prev + data.content);
                        else if (data.type === 'error') {
                            setError(data.message || '分析失败');
                            setLoading(false);
                            return;
                        } else if (data.type === 'done') {
                            setLoading(false);
                            return;
                        }
                    } catch {
                    }
                }
            }
            setLoading(false);
        } catch (e: any) {
            setError(e?.message || '连接失败');
            setLoading(false);
        }
    };

    useEffect(() => {
        return () => {
            try {
                eventRef.current?.cancel();
            } catch {
            }
        };
    }, []);

    const allNewsIds = (latestNews || []).map((n: any) => n.id);
    const allSelected = selectedNewsIds.length > 0 && selectedNewsIds.length === allNewsIds.length;

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-7xl mx-auto space-y-6">
                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <div className="flex items-center justify-between">
                        <div>
                            <h1 className="text-3xl font-bold text-gray-900 mb-2">AI 分析</h1>
                            <p className="text-gray-600">选择交易对，生成提示词，进行大模型分析</p>
                        </div>
                    </div>

                    <div className="mt-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
                        <div>
                            <label className="block text-sm font-semibold text-gray-700 mb-3 flex items-center">
                                <Target className="w-4 h-4 mr-2"/>
                                选择交易对
                            </label>
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

                        <div className="lg:col-span-2">
                            <label
                                className="block text-sm font-semibold text-gray-700 mb-2">选择新闻（可多选，最多10条）</label>
                            <div
                                className="border border-gray-200 rounded-md p-3 bg-gray-50 max-h-40 overflow-auto mb-3">
                                <div className="flex items-center justify-between text-xs text-gray-600 mb-2">
                                    <span>最近新闻</span>
                                    <Button size="sm" variant="outline" className="h-6 px-2 border-black"
                                            onClick={() => setSelectedNewsIds(allSelected ? [] : allNewsIds.slice(0, 10))}>{allSelected ? '全不选' : '全选'}</Button>
                                </div>
                                <div className="grid grid-cols-1 gap-2">
                                    {latestNews?.slice(0, 50).map((n: any) => (
                                        <label key={n.id} className="flex items-start space-x-2 text-sm">
                                            <input type="checkbox" className="mt-1"
                                                   checked={selectedNewsIds.includes(n.id)} onChange={(e) => {
                                                const checked = e.target.checked;
                                                setSelectedNewsIds(checked ? [...selectedNewsIds, n.id] : selectedNewsIds.filter(id => id !== n.id));
                                            }}/>
                                            <span className="truncate">[{n.source}] {n.title}</span>
                                        </label>
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div className="bg-white rounded-lg border-2 border-black p-6">
                    <div className="text-sm font-semibold mb-2">编辑提示词</div>
                    <textarea className="w-full border-2 border-black rounded-md p-3 text-sm min-h-32 mb-3"
                              placeholder="生成或手写你的提示词..." value={promptText}
                              onChange={(e) => setPromptText(e.target.value)}/>
                    <div className={'flex items-center gap-2'}>
                        <Button onClick={buildPrompt} variant="outline" className="border-black">
                            生成提示词
                        </Button>
                        <Button variant="outline" className="border-black" disabled={!promptText.trim()}
                                onClick={startStream}>开始分析</Button>
                    </div>
                </div>

                <Card className="p-6 bg-white border-2 border-black">
                    <h3 className="text-lg font-bold mb-4 text-gray-900 flex items-center">
                        <Bot className="w-5 h-5 mr-2"/>AI 智能分析报告
                        {loading && (
                            <span className="ml-2"><LoadingSpinner/></span>
                        )}
                    </h3>
                    {error ? (
                        <div className="text-center text-red-500 py-12">
                            <Bot className="w-16 h-16 mx-auto mb-4 text-red-300"/>
                            <p className="font-semibold">{error}</p>
                        </div>
                    ) : content || loading ? (
                        <div className="bg-gray-50 rounded-lg p-6 border border-gray-200">
                            <div className="prose prose-slate max-w-none">
                                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                    {content}
                                </ReactMarkdown>
                                {loading && content && (
                                    <span className="inline-block w-2 h-5 bg-blue-500 animate-pulse"></span>
                                )}
                            </div>
                        </div>
                    ) : (
                        <div className="text-center text-gray-500 py-12">
                            <AlertTriangle className="w-16 h-16 mx-auto mb-4 text-gray-300"/>
                            <p>请先生成或填写提示词，再开始分析</p>
                        </div>
                    )}
                </Card>
            </div>
        </div>
    );
}
