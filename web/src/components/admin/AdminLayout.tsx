import {Link, NavLink, Outlet, useNavigate} from 'react-router-dom';

export function AdminLayout() {
    const navigate = useNavigate();
    const navItems = [
        {to: '/admin', label: '管理首页'},
        {to: '/admin/control', label: '系统控制'},
        {to: '/admin/config', label: '系统配置'},
        {to: '/admin/users', label: '用户管理'},
    ];

    const handleLogout = () => {
        localStorage.removeItem('admin_token');
        localStorage.removeItem('admin_user');
        navigate('/login');
    };

    return (
        <div className="min-h-screen bg-gray-50">
            <nav className="bg-white shadow-sm">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex justify-between h-16 gap-6">
                        <div className="flex items-center gap-6">
                            <Link
                                to="/admin"
                                className="text-xl font-bold text-gray-900 hover:text-blue-600 transition-colors"
                            >
                                Prism 管理后台
                            </Link>
                            <div className="hidden md:flex items-center gap-2">
                                {navItems.map((item) => (
                                    <NavLink
                                        key={item.to}
                                        to={item.to}
                                        className={({isActive}) =>
                                            [
                                                'px-3 py-2 rounded-md text-sm font-medium transition-colors',
                                                isActive
                                                    ? 'text-blue-600 bg-blue-50'
                                                    : 'text-gray-700 hover:text-blue-600 hover:bg-blue-50',
                                            ].join(' ')
                                        }
                                        end={item.to === '/admin'}
                                    >
                                        {item.label}
                                    </NavLink>
                                ))}
                            </div>
                        </div>
                        <div className="flex items-center gap-2 sm:gap-4">
                            <Link
                                to="/"
                                className="text-gray-700 hover:text-blue-600 px-3 py-2 rounded-md text-sm font-medium transition-colors"
                            >
                                返回交易首页
                            </Link>
                            <button
                                onClick={handleLogout}
                                className="bg-red-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-red-700 transition-colors"
                            >
                                退出登录
                            </button>
                        </div>
                    </div>
                    <div className="flex md:hidden items-center gap-2 pb-4">
                        {navItems.map((item) => (
                            <NavLink
                                key={item.to}
                                to={item.to}
                                className={({isActive}) =>
                                    [
                                        'px-3 py-2 rounded-md text-sm font-medium transition-colors',
                                        isActive
                                            ? 'text-blue-600 bg-blue-50'
                                            : 'text-gray-700 hover:text-blue-600 hover:bg-blue-50',
                                    ].join(' ')
                                }
                                end={item.to === '/admin'}
                            >
                                {item.label}
                            </NavLink>
                        ))}
                    </div>
                </div>
            </nav>

            <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
                <Outlet />
            </main>
        </div>
    );
}
