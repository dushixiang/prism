import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export function Setup() {
    const navigate = useNavigate();
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [nickname, setNickname] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');

        // 前端验证
        if (!username || !password) {
            setError('用户名和密码不能为空');
            return;
        }

        if (password.length < 5) {
            setError('密码长度至少5位');
            return;
        }

        if (password !== confirmPassword) {
            setError('两次输入的密码不一致');
            return;
        }

        setLoading(true);

        try {
            const response = await fetch('/api/setup/init', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    username,
                    password,
                    nickname: nickname || username,
                }),
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || '初始化失败');
            }

            // 初始化成功，跳转到登录页
            navigate('/login');
        } catch (err) {
            setError(err instanceof Error ? err.message : '初始化失败');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
            <div className="w-full max-w-md bg-white rounded-lg shadow-lg p-8">
                <div className="space-y-2 mb-6">
                    <h1 className="text-2xl font-bold text-center">
                        🚀 欢迎使用 Prism
                    </h1>
                    <p className="text-center text-gray-600">
                        首次使用需要创建管理员账号
                    </p>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    {error && (
                        <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded">
                            {error}
                        </div>
                    )}

                    <div className="space-y-2">
                        <label htmlFor="username" className="block text-sm font-medium text-gray-700">
                            用户名 *
                        </label>
                        <input
                            id="username"
                            type="text"
                            placeholder="请输入用户名"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            disabled={loading}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div className="space-y-2">
                        <label htmlFor="nickname" className="block text-sm font-medium text-gray-700">
                            昵称
                        </label>
                        <input
                            id="nickname"
                            type="text"
                            placeholder="留空则使用用户名"
                            value={nickname}
                            onChange={(e) => setNickname(e.target.value)}
                            disabled={loading}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div className="space-y-2">
                        <label htmlFor="password" className="block text-sm font-medium text-gray-700">
                            密码 *
                        </label>
                        <input
                            id="password"
                            type="password"
                            placeholder="至少6位"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            disabled={loading}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div className="space-y-2">
                        <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-700">
                            确认密码 *
                        </label>
                        <input
                            id="confirmPassword"
                            type="password"
                            placeholder="请再次输入密码"
                            value={confirmPassword}
                            onChange={(e) => setConfirmPassword(e.target.value)}
                            disabled={loading}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="cursor-pointer w-full bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                    >
                        {loading ? '初始化中...' : '创建管理员账号'}
                    </button>

                    <p className="text-xs text-center text-gray-500 mt-4">
                        创建后请妥善保管管理员账号信息
                    </p>
                </form>
            </div>
        </div>
    );
}
