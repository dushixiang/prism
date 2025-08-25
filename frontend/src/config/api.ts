// API 配置
export const API_CONFIG = {
  // 开发环境配置
  development: {
    baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api',
    timeout: parseInt(import.meta.env.VITE_API_TIMEOUT || '30000'),
  },
  // 生产环境配置
  production: {
    baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
    timeout: parseInt(import.meta.env.VITE_API_TIMEOUT || '30000'),
  },
};

// 获取当前环境的API配置
export function getApiConfig() {
  const isDevelopment = import.meta.env.DEV || process.env.NODE_ENV === 'development';
  
  return isDevelopment ? API_CONFIG.development : API_CONFIG.production;
}

// 导出当前配置
export const apiConfig = getApiConfig();

// 打印当前配置（仅开发环境）
if (import.meta.env.DEV) {
  console.log('🔧 API配置:', apiConfig);
  console.log('🌍 当前环境:', import.meta.env.DEV ? '开发' : '生产');
} 