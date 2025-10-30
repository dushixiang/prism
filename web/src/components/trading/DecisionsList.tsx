import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';
import {formatCurrency, formatDateTime, getErrorMessage} from '@/utils/formatters.ts';
import {cardClass} from '@/constants/styles.ts';
import {LLMLogViewer} from '@/components/llm/LLMLogViewer.tsx';
import type {Decision} from '@/types/trading.ts';

interface DecisionsListProps {
    decisions: Decision[] | undefined;
    error: unknown;
}

export const DecisionsList = ({decisions, error}: DecisionsListProps) => {
    if (error) {
        return <p className="text-sm text-rose-500">{getErrorMessage(error)}</p>;
    }

    if (!decisions || decisions.length === 0) {
        return <p className="text-sm text-slate-500">暂无决策记录</p>;
    }

    return (
        <>
            {decisions.map((decision) => (
                <div
                    key={decision.id}
                    className={`${cardClass} mb-3 p-3 sm:p-4`}
                >
                    <div
                        className="mb-2 flex items-center justify-between text-xs text-slate-600">
                        <span>第 {decision.iteration} 次迭代</span>
                        <span>{formatDateTime(decision.executed_at)}</span>
                    </div>
                    <div
                        className="prose prose-sm prose-slate max-w-none text-sm [&>*]:mb-2 [&>*:last-child]:mb-0 [&_p]:leading-relaxed [&_ul]:my-2 [&_ol]:my-2 [&_li]:my-1 [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-sm [&_h4]:text-xs [&_strong]:font-semibold [&_code]:bg-slate-100 [&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_code]:text-xs [&_pre]:bg-slate-100 [&_pre]:p-2 [&_pre]:rounded [&_pre]:overflow-x-auto [&_table]:w-full [&_table]:border-collapse [&_table]:my-3 [&_table]:text-xs [&_th]:border [&_th]:border-slate-300 [&_th]:bg-slate-100 [&_th]:px-2 [&_th]:py-1.5 [&_th]:text-left [&_th]:font-semibold [&_td]:border [&_td]:border-slate-300 [&_td]:px-2 [&_td]:py-1.5">
                        <ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]}>
                            {decision.decision_content ?? ''}
                        </ReactMarkdown>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                        <span>账户: {formatCurrency(decision.account_value)}</span>
                        <span>持仓: {decision.position_count}</span>
                        <span>令牌: {decision.prompt_tokens}/{decision.completion_tokens}</span>
                    </div>

                    {/* LLM 日志查看器 */}
                    <LLMLogViewer decisionId={decision.id}/>
                </div>
            ))}
        </>
    );
};
