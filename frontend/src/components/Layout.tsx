import {useEffect} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Link, useLocation, useNavigate} from 'react-router-dom';
import {Logo} from './ui/logo';
import {authApi} from '../services/api';
import {tokenManager} from '../utils/token';
import {Brain, Home, Newspaper, TrendingUp,} from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger
} from './ui/dropdown-menu';

interface LayoutProps {
    children: React.ReactNode;
}

export function Layout({children}: LayoutProps) {
    const location = useLocation();
    const navigate = useNavigate();

    const {data: userProfile, isLoading: profileLoading} = useQuery({
        queryKey: ['user-profile'],
        queryFn: () => authApi.getProfile(),
        retry: false,
        refetchOnWindowFocus: false,
        enabled: tokenManager.hasToken(),
    });

    const user = userProfile;

    useEffect(() => {
        const hasToken = tokenManager.hasToken();
        if (!hasToken && location.pathname !== '/login') {
            navigate('/login');
        } else if (!profileLoading && !user && hasToken && location.pathname !== '/login') {
            navigate('/login');
        }
    }, [user, profileLoading, location.pathname, navigate]);

    const menuItems = [
        {path: '/dashboard', name: '仪表盘', icon: Home},
        {path: '/market-analysis', name: '技术分析', icon: TrendingUp},
        {path: '/ai-analysis', name: 'AI 分析', icon: Brain},
        {path: '/news', name: '新闻资讯', icon: Newspaper},
    ];

    if (location.pathname === '/login') return <>{children}</>;
    if (profileLoading) {
        return (
            <div className="flex items-center justify-center min-h-screen bg-gray-50">
                <div className="text-center">
                    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
                    <p className="text-gray-600">加载中...</p>
                </div>
            </div>
        );
    }
    if (!user) return null;

    const handleLogout = async () => {
        try {
            await authApi.logout();
        } catch {
        } finally {
            tokenManager.removeToken();
            navigate('/login');
        }
    };

    return (
        <div className="min-h-screen bg-gray-50 flex flex-col">
            {/* 顶部导航 */}
            <header className="w-full bg-white border-b-2 border-black">
                <div className="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <Logo size={20}/>
                        <span className="text-lg font-bold text-gray-900">Prism</span>
                    </div>

                    <nav className="flex items-center gap-1">
                        {menuItems.map((item) => {
                            const isActive = location.pathname === item.path;
                            const IconComponent = item.icon as any;
                            return (
                                <Link
                                    key={item.path}
                                    to={item.path}
                                    className={`group relative flex items-center gap-2 px-3 h-10 transition-colors text-sm font-medium ${
                                        isActive ? 'text-gray-900' : 'text-gray-700 hover:text-gray-900'
                                    }`}
                                >
                                    <IconComponent className="w-4 h-4"/>
                                    <span className="whitespace-nowrap">{item.name}</span>
                                    <span
                                        className={`absolute left-2 right-2 -bottom-0.5 h-[2px] ${isActive ? 'bg-black' : 'bg-transparent group-hover:bg-gray-300'}`}></span>
                                </Link>
                            );
                        })}
                    </nav>

                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <button className="flex items-center gap-2 cursor-pointer select-none">
                                <div
                                    className="w-8 h-8 bg-gradient-to-br from-blue-500 to-blue-600 rounded-full flex items-center justify-center">
                                    <span
                                        className="text-white text-sm font-medium">{user.name.charAt(0).toUpperCase()}</span>
                                </div>
                                <div
                                    className="hidden sm:block text-sm text-gray-800 max-w-[160px] truncate">{user.name}</div>
                            </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="border-2 border-black">
                            <DropdownMenuItem onClick={() => navigate('/account/profile')}>个人信息</DropdownMenuItem>
                            <DropdownMenuItem
                                onClick={() => navigate('/account/change-password')}>修改密码</DropdownMenuItem>
                            <DropdownMenuSeparator/>
                            <DropdownMenuItem data-variant="destructive"
                                              onClick={handleLogout}>退出登录</DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </header>

            {/* 主内容 */}
            <main className="flex-1">
                <div className="max-w-7xl mx-auto px-4 py-6">
                    {children}
                </div>
            </main>

            {/* 底部占位，可后续扩展 */}
            <footer className="w-full border-t-2 border-black bg-white">
                <div className="max-w-7xl mx-auto px-4 py-2 text-xs text-gray-500">
                    Prism © {new Date().getFullYear()}
                </div>
            </footer>
        </div>
    );
}