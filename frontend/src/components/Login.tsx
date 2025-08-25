import {useState} from 'react';
import {useNavigate} from 'react-router-dom';
import {useMutation} from '@tanstack/react-query';
import {authApi} from '../services/api';
import type {LoginRequest} from '@/types';

export function Login() {
    const [formData, setFormData] = useState<LoginRequest>({
        account: '',
        password: ''
    });
    const [error, setError] = useState('');
    const navigate = useNavigate();

    const loginMutation = useMutation({
        mutationFn: authApi.login,
        onSuccess: (response) => {
            // 检查登录是否成功并且有token
            if (response.token) {
                navigate('/dashboard');
                window.location.reload(); // 刷新获取用户信息
            } else {
                setError('登录失败：未收到有效token');
            }
        },
        onError: (error: any) => {
            setError(error?.message || '登录失败，请检查用户名和密码');
        }
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        setError('');

        if (!formData.account || !formData.password) {
            setError('请输入用户名和密码');
            return;
        }

        loginMutation.mutate(formData);
    };

    const handleInputChange = (field: keyof LoginRequest, value: string) => {
        setFormData(prev => ({...prev, [field]: value}));
        if (error) setError(''); // 清除错误信息
    };

    return (
        <div className="min-h-screen bg-white flex items-center justify-center p-4">
            <div className="w-full max-w-md">
                {/* 登录卡片 */}
                <div className="bg-white border-2 border-black rounded-lg shadow-lg p-8">
                    {/* Logo和标题 */}
                    <div className="text-center mb-8">
                        <div className="flex items-center justify-center mb-4">
                            <div className="w-12 h-12 bg-black rounded-lg flex items-center justify-center">
                                <span className="text-white text-xl font-bold">P</span>
                            </div>
                        </div>
                        <h1 className="text-2xl font-bold text-black">Prism</h1>
                        <p className="text-gray-600 mt-1 text-sm">加密货币智能分析平台</p>
                    </div>

                    {/* 登录表单 */}
                    <form onSubmit={handleSubmit} className="space-y-6">
                        <div>
                            <label className="block text-sm font-medium text-black mb-2">
                                用户名
                            </label>
                            <input
                                type="text"
                                value={formData.account}
                                onChange={(e) => handleInputChange('account', e.target.value)}
                                className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-black focus:outline-none transition-colors"
                                placeholder="请输入用户名"
                                disabled={loginMutation.isPending}
                            />
                        </div>

                        <div>
                            <label className="block text-sm font-medium text-black mb-2">
                                密码
                            </label>
                            <input
                                type="password"
                                value={formData.password}
                                onChange={(e) => handleInputChange('password', e.target.value)}
                                className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-black focus:outline-none transition-colors"
                                placeholder="请输入密码"
                                disabled={loginMutation.isPending}
                            />
                        </div>

                        {error && (
                            <div className="p-3 bg-red-50 border-2 border-red-200 rounded-lg">
                                <p className="text-sm text-red-600">{error}</p>
                            </div>
                        )}

                        <button
                            type="submit"
                            className="w-full bg-black text-white py-3 px-4 rounded-lg font-medium hover:bg-gray-800 transition-colors disabled:bg-gray-400 disabled:cursor-not-allowed"
                            disabled={loginMutation.isPending}
                        >
                            {loginMutation.isPending ? (
                                <div className="flex items-center justify-center space-x-2">
                                    <div
                                        className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                                    <span>登录中...</span>
                                </div>
                            ) : (
                                '登录'
                            )}
                        </button>
                    </form>

                    {/* 功能特色 */}
                    <div className="mt-8 pt-6 border-t-2 border-gray-100">
                        <div className="text-center">
                            <p className="text-sm font-medium text-black mb-3">✨ 核心功能</p>
                            <div className="grid grid-cols-2 gap-2 text-xs text-gray-600">
                                <div className="flex items-center space-x-1">
                                    <span>📊</span>
                                    <span>技术分析</span>
                                </div>
                                <div className="flex items-center space-x-1">
                                    <span>🤖</span>
                                    <span>AI预测</span>
                                </div>
                                <div className="flex items-center space-x-1">
                                    <span>💼</span>
                                    <span>投资组合</span>
                                </div>
                                <div className="flex items-center space-x-1">
                                    <span>📈</span>
                                    <span>交易信号</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* 底部版权 */}
                <div className="text-center mt-6">
                    <p className="text-xs text-gray-500">© 2024 Prism • 专业数字资产分析平台</p>
                </div>
            </div>
        </div>
    );
}
