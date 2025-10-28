// Token管理工具
const TOKEN_KEY = 'prism-token';

export const tokenManager = {
    // 获取token
    getToken(): string | null {
        return localStorage.getItem(TOKEN_KEY);
    },

    // 设置token
    setToken(token: string): void {
        localStorage.setItem(TOKEN_KEY, token);
    },

    // 删除token
    removeToken(): void {
        localStorage.removeItem(TOKEN_KEY);
    },

    // 检查是否有token
    hasToken(): boolean {
        return !!this.getToken();
    }
};
