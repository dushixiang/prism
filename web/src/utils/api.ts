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
