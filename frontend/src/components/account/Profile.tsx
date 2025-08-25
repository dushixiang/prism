import {useState} from 'react';
import {authApi} from '../../services/api';
import {Card} from '../ui/card';
import {Button} from '../ui/button';

export function ProfilePage() {
    const [name, setName] = useState('');
    const [avatar, setAvatar] = useState('');
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setSaving(true);
        setMessage(null);
        try {
            await authApi.changeProfile({ name: name || undefined, avatar: avatar || undefined });
            setMessage('个人信息已更新');
        } catch (e: any) {
            setMessage(e?.message || '更新失败');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="min-h-screen bg-gray-50 p-6">
            <div className="max-w-2xl mx-auto">
                <Card className="p-6 bg-white border-2 border-black">
                    <h1 className="text-2xl font-bold mb-4 text-gray-900">修改个人信息</h1>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">昵称</label>
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                className="w-full px-3 py-2 border-2 border-gray-300 rounded-md focus:border-black focus:outline-none"
                                placeholder="输入新的昵称"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">头像URL（可选）</label>
                            <input
                                type="url"
                                value={avatar}
                                onChange={(e) => setAvatar(e.target.value)}
                                className="w-full px-3 py-2 border-2 border-gray-300 rounded-md focus:border-black focus:outline-none"
                                placeholder="https://example.com/avatar.png"
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
