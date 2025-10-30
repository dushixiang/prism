import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from '../ui/sheet';
import {fetcher} from '@/utils/api.ts';
import {getErrorMessage} from '@/utils/formatters.ts';
import type {LLMLogsResponse} from '@/types/trading.ts';

interface LLMLogViewerProps {
    decisionId: string;
}

export const LLMLogViewer = ({decisionId}: LLMLogViewerProps) => {
    const [isOpen, setIsOpen] = useState(false);
    const [selectedRound, setSelectedRound] = useState<number | null>(null);

    const {
        data: logsData,
        isLoading,
        error,
    } = useQuery<LLMLogsResponse>({
        queryKey: ['llm-logs', decisionId],
        queryFn: () => fetcher<LLMLogsResponse>(`/api/trading/llm-logs?decision_id=${decisionId}`),
        enabled: isOpen,
    });

    return (
        <Sheet open={isOpen} onOpenChange={setIsOpen}>
            <SheetTrigger asChild>
                <button
                    className="mt-2 w-full rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 transition hover:bg-blue-100"
                >
                    🔍 查看 LLM 通信日志
                </button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full p-0 sm:max-w-[600px] lg:max-w-[800px]">
                <SheetHeader className="border-b border-slate-200 px-6 py-4">
                    <SheetTitle>LLM 通信日志</SheetTitle>
                    <SheetDescription>
                        {logsData?.logs ? `共 ${logsData.logs.length} 轮对话` : '加载中...'}
                    </SheetDescription>
                </SheetHeader>

                <div className="h-[calc(100vh-80px)] overflow-y-auto px-6 py-4">
                    {isLoading && (
                        <div className="flex h-full items-center justify-center text-slate-500">
                            加载中...
                        </div>
                    )}

                    {error && (
                        <div className="rounded bg-rose-50 p-4 text-sm text-rose-600">
                            {getErrorMessage(error)}
                        </div>
                    )}

                    {logsData?.logs && logsData.logs.length > 0 && (
                        <div className="space-y-3">
                            {logsData.logs.map((log) => (
                                <div
                                    key={log.id}
                                    className={`rounded-lg border ${
                                        selectedRound === log.round_number
                                            ? 'border-blue-300 bg-blue-50'
                                            : 'border-slate-200 bg-white'
                                    } overflow-hidden transition-all`}
                                >
                                    {/* 轮次标题 */}
                                    <button
                                        onClick={() =>
                                            setSelectedRound(selectedRound === log.round_number ? null : log.round_number)
                                        }
                                        className="flex w-full items-center justify-between p-3 text-left hover:bg-slate-50"
                                    >
                                        <div className="flex items-center gap-3">
                                            <span className="flex h-6 w-6 items-center justify-center rounded-full bg-blue-500 text-xs font-bold text-white">
                                                {log.round_number}
                                            </span>
                                            <div className="flex items-center gap-2 text-xs text-slate-600">
                                                <span className="font-mono">{log.duration}ms</span>
                                                <span>•</span>
                                                <span className="font-mono">{log.total_tokens} tokens</span>
                                            </div>
                                        </div>
                                        <svg
                                            className={`h-4 w-4 text-slate-400 transition-transform ${
                                                selectedRound === log.round_number ? 'rotate-180' : ''
                                            }`}
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                        >
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7"/>
                                        </svg>
                                    </button>

                                    {/* 展开内容 */}
                                    {selectedRound === log.round_number && (
                                        <div className="border-t border-slate-200 p-4">
                                            <div className="space-y-3">
                                                {(() => {
                                                    // 检查是否有实际内容
                                                    const hasSystemPrompt = log.round_number === 1 && log.system_prompt;
                                                    const hasUserPrompt = log.round_number === 1 && log.user_prompt;
                                                    const hasAssistantContent = log.assistant_content && log.assistant_content.trim();
                                                    const hasToolCalls = log.tool_calls && log.tool_calls !== '[]';
                                                    const hasToolResponses = log.tool_responses && log.tool_responses !== '[]';
                                                    const hasContent = hasSystemPrompt || hasUserPrompt || hasAssistantContent || hasToolCalls || hasToolResponses;

                                                    if (!hasContent && !log.error) {
                                                        return (
                                                            <div className="rounded bg-slate-50 p-4 text-center text-xs text-slate-500">
                                                                <div className="mb-1">✓ AI 已完成响应</div>
                                                                <div className="text-slate-400">本轮无额外输出内容</div>
                                                            </div>
                                                        );
                                                    }

                                                    return null;
                                                })()}

                                                {/* 系统提示词 */}
                                                {log.round_number === 1 && log.system_prompt && (
                                                    <div>
                                                        <div className="mb-2 text-xs font-semibold text-slate-700">系统提示词</div>
                                                        <details className="group">
                                                            <summary className="cursor-pointer text-xs text-blue-600 hover:text-blue-800">
                                                                点击展开查看 ({log.system_prompt.length} 字符)
                                                            </summary>
                                                            <div className="mt-2 max-h-60 overflow-y-auto whitespace-pre-wrap rounded bg-slate-100 p-3 text-xs text-slate-700">
                                                                {log.system_prompt}
                                                            </div>
                                                        </details>
                                                    </div>
                                                )}

                                                {/* 用户提示词 */}
                                                {log.round_number === 1 && log.user_prompt && (
                                                    <div>
                                                        <div className="mb-2 text-xs font-semibold text-slate-700">用户提示词</div>
                                                        <details className="group">
                                                            <summary className="cursor-pointer text-xs text-blue-600 hover:text-blue-800">
                                                                点击展开查看 ({log.user_prompt.length} 字符)
                                                            </summary>
                                                            <div className="mt-2 max-h-60 overflow-y-auto whitespace-pre-wrap rounded bg-slate-100 p-3 text-xs text-slate-700">
                                                                {log.user_prompt}
                                                            </div>
                                                        </details>
                                                    </div>
                                                )}

                                                {/* AI 思考 */}
                                                {log.assistant_content && log.assistant_content.trim() && (
                                                    <div>
                                                        <div className="mb-2 text-xs font-semibold text-slate-700">AI 思考</div>
                                                        <div className="whitespace-pre-wrap rounded bg-blue-50 p-3 text-xs text-slate-700">
                                                            {log.assistant_content}
                                                        </div>
                                                    </div>
                                                )}

                                                {/* 工具调用 */}
                                                {log.tool_calls && log.tool_calls !== '[]' && (
                                                    <div>
                                                        <div className="mb-2 text-xs font-semibold text-slate-700">工具调用</div>
                                                        <div className="space-y-2">
                                                            {(() => {
                                                                try {
                                                                    const calls = JSON.parse(log.tool_calls);
                                                                    return calls.map((call: any, idx: number) => (
                                                                        <div
                                                                            key={idx}
                                                                            className="rounded bg-amber-50 p-3"
                                                                        >
                                                                            <div className="mb-1 font-semibold text-amber-700">
                                                                                📞 {call.function}
                                                                            </div>
                                                                            <pre className="overflow-x-auto text-xs text-amber-900">
                                                                                {JSON.stringify(call.arguments, null, 2)}
                                                                            </pre>
                                                                        </div>
                                                                    ));
                                                                } catch {
                                                                    return <div className="text-xs text-slate-500">解析失败</div>;
                                                                }
                                                            })()}
                                                        </div>
                                                    </div>
                                                )}

                                                {/* 工具响应 */}
                                                {log.tool_responses && log.tool_responses !== '[]' && (
                                                    <div>
                                                        <div className="mb-2 text-xs font-semibold text-slate-700">工具响应</div>
                                                        <div className="space-y-2">
                                                            {(() => {
                                                                try {
                                                                    const responses = JSON.parse(log.tool_responses);
                                                                    return responses.map((response: any, idx: number) => (
                                                                        <div
                                                                            key={idx}
                                                                            className={`rounded p-3 ${
                                                                                response.error
                                                                                    ? 'bg-rose-50 text-rose-900'
                                                                                    : 'bg-emerald-50 text-emerald-900'
                                                                            }`}
                                                                        >
                                                                            <pre className="overflow-x-auto text-xs">
                                                                                {JSON.stringify(response.result || response, null, 2)}
                                                                            </pre>
                                                                        </div>
                                                                    ));
                                                                } catch {
                                                                    return <div className="text-xs text-slate-500">解析失败</div>;
                                                                }
                                                            })()}
                                                        </div>
                                                    </div>
                                                )}

                                                {/* Token统计 */}
                                                <div className="flex flex-wrap gap-3 border-t border-slate-200 pt-3 text-xs text-slate-600">
                                                    <span>输入: {log.prompt_tokens}</span>
                                                    <span>输出: {log.completion_tokens}</span>
                                                    <span>总计: {log.total_tokens}</span>
                                                    <span>耗时: {log.duration}ms</span>
                                                    {log.finish_reason && <span>结束: {log.finish_reason}</span>}
                                                </div>

                                                {/* 错误信息 */}
                                                {log.error && (
                                                    <div className="rounded bg-rose-50 p-3 text-xs text-rose-700">
                                                        ❌ {log.error}
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    )}
                                </div>
                            ))}
                        </div>
                    )}

                    {logsData?.logs && logsData.logs.length === 0 && (
                        <div className="flex h-full items-center justify-center text-slate-400">
                            暂无日志记录
                        </div>
                    )}
                </div>
            </SheetContent>
        </Sheet>
    );
};
