import {useState} from 'react';

export function UserManagement() {
    const [currentUser] = useState(() => {
        const user = localStorage.getItem('admin_user');
        return user ? JSON.parse(user) : null;
    });

    const [oldPassword, setOldPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    const handleChangePassword = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setSuccess('');

        if (!oldPassword || !newPassword) {
            setError('请填写所有必填字段');
            return;
        }

        if (newPassword.length < 5) {
            setError('新密码长度至少5位');
            return;
        }

        if (newPassword !== confirmPassword) {
            setError('两次输入的新密码不一致');
            return;
        }

        setLoading(true);

        try {
            const token = localStorage.getItem('admin_token');
            const response = await fetch('/api/auth/change-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify({
                    old_password: oldPassword,
                    new_password: newPassword,
                }),
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || '修改密码失败');
            }

            setSuccess('密码修改成功');
            setOldPassword('');
            setNewPassword('');
            setConfirmPassword('');
        } catch (err) {
            setError(err instanceof Error ? err.message : '修改密码失败');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-2xl font-bold mb-4">用户管理</h2>
                <p className="text-gray-600">管理当前用户信息和密码</p>
            </div>

            {/* 当前用户信息 */}
            <div className="bg-white rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold mb-4">当前用户信息</h3>
                <div className="space-y-3">
                    <div className="flex">
                        <span className="text-gray-600 w-24">用户名:</span>
                        <span className="font-medium">{currentUser?.username || '-'}</span>
                    </div>
                    <div className="flex">
                        <span className="text-gray-600 w-24">昵称:</span>
                        <span className="font-medium">{currentUser?.nickname || '-'}</span>
                    </div>
                    <div className="flex">
                        <span className="text-gray-600 w-24">角色:</span>
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                            {currentUser?.role || 'admin'}
                        </span>
                    </div>
                </div>
            </div>

            {/* 修改密码 */}
            <div className="bg-white rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold mb-4">修改密码</h3>
                <form onSubmit={handleChangePassword} className="space-y-4 max-w-md">
                    {error && (
                        <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded">
                            {error}
                        </div>
                    )}

                    {success && (
                        <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded">
                            {success}
                        </div>
                    )}

                    <div>
                        <label htmlFor="oldPassword" className="block text-sm font-medium text-gray-700 mb-2">
                            旧密码 *
                        </label>
                        <input
                            id="oldPassword"
                            type="password"
                            value={oldPassword}
                            onChange={(e) => setOldPassword(e.target.value)}
                            disabled={loading}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div>
                        <label htmlFor="newPassword" className="block text-sm font-medium text-gray-700 mb-2">
                            新密码 * (至少5位)
                        </label>
                        <input
                            id="newPassword"
                            type="password"
                            value={newPassword}
                            onChange={(e) => setNewPassword(e.target.value)}
                            disabled={loading}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div>
                        <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-700 mb-2">
                            确认新密码 *
                        </label>
                        <input
                            id="confirmPassword"
                            type="password"
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
                        className="bg-blue-600 text-white px-6 py-2 rounded-md hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
                    >
                        {loading ? '提交中...' : '修改密码'}
                    </button>
                </form>
            </div>

            {/* 安全提示 */}
            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
                <h4 className="font-semibold text-yellow-800 mb-2">安全提示</h4>
                <ul className="text-sm text-yellow-700 space-y-1 list-disc list-inside">
                    <li>建议定期修改密码以保证账号安全</li>
                    <li>密码应包含字母、数字和特殊字符</li>
                    <li>不要使用过于简单或容易被猜到的密码</li>
                    <li>不要与他人分享您的账号密码</li>
                </ul>
            </div>
        </div>
    );
}
