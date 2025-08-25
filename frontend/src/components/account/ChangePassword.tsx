import {useState} from 'react';
import {authApi} from '../../services/api';
import {Card} from '../ui/card';
import {Button} from '../ui/button';

export function ChangePasswordPage() {
    const [oldPassword, setOldPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setMessage(null);
        if (!newPassword || newPassword !== confirmPassword) {
            setMessage('两次输入的新密码不一致');
            return;
        }
        setSaving(true);
        try {
            await authApi.changePassword({ old_password: oldPassword, new_password: newPassword });
            setMessage('密码已修改');
            setOldPassword('');
            setNewPassword('');
            setConfirmPassword('');
        } catch (e: any) {
            setMessage(e?.message || '修改失败');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-2xl mx-auto">
                <Card className="p-6 bg-white border-2 border-black">
                    <h1 className="text-2xl font-bold mb-4 text-gray-900">修改密码</h1>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">当前密码</label>
                            <input
                                type="password"
                                value={oldPassword}
                                onChange={(e) => setOldPassword(e.target.value)}
                                className="w-full px-3 py-2 border-2 border-gray-300 rounded-md focus:border-black focus:outline-none"
                                placeholder="输入当前密码"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">新密码</label>
                            <input
                                type="password"
                                value={newPassword}
                                onChange={(e) => setNewPassword(e.target.value)}
                                className="w-full px-3 py-2 border-2 border-gray-300 rounded-md focus:border-black focus:outline-none"
                                placeholder="输入新密码"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">确认新密码</label>
                            <input
                                type="password"
                                value={confirmPassword}
                                onChange={(e) => setConfirmPassword(e.target.value)}
                                className="w-full px-3 py-2 border-2 border-gray-300 rounded-md focus:border-black focus:outline-none"
                                placeholder="再次输入新密码"
                            />
                        </div>
                        <div className="flex items-center gap-3">
                            <Button type="submit" disabled={saving} className="border-black" variant="outline">
                                {saving ? '保存中...' : '保存'}
                            </Button>
                            {message && <span className="text-sm text-gray-600">{message}</span>}
                        </div>
                    </form>
                </Card>
            </div>
        </div>
    );
}
