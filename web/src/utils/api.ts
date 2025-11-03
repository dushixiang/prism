import {QueryClient} from '@tanstack/react-query';

export const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            retry: 1,
            refetchOnWindowFocus: false,
            staleTime: 15000,
        },
    },
});

export const fetcher = async <T, >(url: string): Promise<T> => {
    const response = await fetch(url);
    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || '请求失败');
    }
    return response.json() as Promise<T>;
};

// 交易系统控制 API
export const tradingControlAPI = {
    // 启动交易系统
    start: async () => {
        const response = await fetch('/api/trading/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '启动失败');
        }
        return response.json();
    },

    // 停止交易系统
    stop: async () => {
        const response = await fetch('/api/trading/stop', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '停止失败');
        }
        return response.json();
    },

    // 重启交易系统
    restart: async () => {
        const response = await fetch('/api/trading/restart', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '重启失败');
        }
        return response.json();
    },
};
