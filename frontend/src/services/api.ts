import type {
    AnalyzeSymbolRequest,
    ErrorResponse,
    KlineData,
    LoginRequest,
    LoginResponse,
    MarketAnalysis,
    News,
    NewsStatistics,
    TechnicalIndicators,
    TradingSignal,
    UserInfo
} from '@/types';
import {tokenManager} from '../utils/token';

const API_BASE = '/api';

// 通用请求函数
async function fetchApi<T>(
    endpoint: string,
    options: RequestInit = {}
): Promise<T> {
    // 准备headers
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...options.headers as Record<string, string>,
    };

    // 添加token header（如果有token且不是登录接口）
    const token = tokenManager.getToken();
    if (token && endpoint !== '/login') {
        headers['Prism-Token'] = token;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
        credentials: 'include',
        headers,
        ...options,
    });

    const data = await response.json();

    if (!response.ok) {
        // 如果是401未授权，清除token
        if (response.status === 401) {
            tokenManager.removeToken();
        }

        // 处理错误响应格式 {code: xxx, message: "xxx"}
        const errorData = data as ErrorResponse;
        throw new Error(errorData.message || `HTTP error! status: ${response.status}`);
    }

    // 正常响应直接返回数据，不包装
    return data;
}

// ===== 认证API =====
export const authApi = {
    login: async (data: LoginRequest) => {
        const response = await fetchApi<LoginResponse>('/login', {
            method: 'POST',
            body: JSON.stringify(data),
        });

        // 登录成功后保存token
        if (response.token) {
            tokenManager.setToken(response.token);
        }

        return response;
    },

    logout: () => {
        // 清除本地token
        tokenManager.removeToken();
        return fetchApi('/account/logout', {method: 'POST'});
    },

    getProfile: () =>
        fetchApi<UserInfo>('/account/info'),

    changePassword: (data: { old_password: string; new_password: string }) =>
        fetchApi('/account/change-password', {
            method: 'POST',
            body: JSON.stringify(data),
        }),

    changeProfile: (data: { name?: string; avatar?: string }) =>
        fetchApi('/account/change-profile', {
            method: 'POST',
            body: JSON.stringify(data),
        }),
};

// ===== 市场分析API =====
export const marketApi = {
    analyzeSymbol: (data: AnalyzeSymbolRequest) =>
        fetchApi<{
            symbol: string;
            interval: string;
            analysis: MarketAnalysis;
            technical_indicators: TechnicalIndicators;
            trading_signal: TradingSignal;
        }>('/market/analyze/symbol', {
            method: 'POST',
            body: JSON.stringify(data),
        }),

    getKlineData: (symbol: string, interval: string, limit?: number) => {
        const params = new URLSearchParams({symbol, interval});
        if (limit) params.append('limit', limit.toString());

        return fetchApi<{ symbol: string, interval: string, kline_data: KlineData[] }>(`/market/kline?${params}`);
    },

    getSymbols: () =>
        fetchApi<{ symbols: string[] }>(`/market/symbols`),

    buildPrompt: (data: { symbol: string; interval: string; limit?: number; news_ids?: string[] }) =>
        fetchApi<{ prompt: string }>(`/market/llm/prompt`, {
            method: 'POST',
            body: JSON.stringify(data),
        }),

    getMarketOverview: () =>
        fetchApi<any>('/market/overview'),

    getTrendingSymbols: (limit?: number) => {
        const params = limit ? `?limit=${limit}` : '';
        return fetchApi<any[]>(`/market/trending${params}`);
    },
};


// ===== 新闻资讯API =====
export const newsApi = {

    getLatest: (params: URLSearchParams) => {
        let s = params.toString();
        return fetchApi<News[]>(`/news/latest?${s}`);
    },

    getStatistics: () => {
        return fetchApi<NewsStatistics>('/news/statistics');
    },
};

// 导出所有API
export default {
    auth: authApi,
    market: marketApi,
    news: newsApi,
};