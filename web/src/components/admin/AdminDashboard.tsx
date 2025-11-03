import {Link} from 'react-router-dom';

export function AdminDashboard() {
    const menuItems = [
        {
            title: '系统控制',
            description: '启动、停止、重启交易系统',
            icon: '🎮',
            link: '/admin/control',
        },
        {
            title: '系统配置',
            description: '管理系统提示词、交易参数等配置',
            icon: '⚙️',
            link: '/admin/config',
        },
        {
            title: '用户管理',
            description: '管理当前用户信息和密码',
            icon: '👤',
            link: '/admin/users',
        },
    ];

    return (
        <div className="px-4 py-6 sm:px-0">
            <div className="bg-white rounded-lg shadow p-6">
                <h2 className="text-2xl font-bold mb-4">欢迎使用管理后台</h2>
                <p className="text-gray-600 mb-4">
                    您已成功登录管理后台。选择下方功能进行管理。
                </p>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mt-6">
                    {menuItems.map((item) => (
                        <Link
                            key={item.link}
                            to={item.link}
                            className="border rounded-lg p-6 hover:shadow-md hover:border-blue-500 transition-all"
                        >
                            <div className="text-4xl mb-3">{item.icon}</div>
                            <h3 className="font-semibold text-lg mb-2">{item.title}</h3>
                            <p className="text-sm text-gray-600">{item.description}</p>
                        </Link>
                    ))}
                </div>
            </div>
        </div>
    );
}
