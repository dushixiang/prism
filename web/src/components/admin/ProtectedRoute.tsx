import {useEffect, useState} from 'react';
import {Navigate, useLocation} from 'react-router-dom';

interface ProtectedRouteProps {
    children: React.ReactNode;
}

export function ProtectedRoute({children}: ProtectedRouteProps) {
    const [isChecking, setIsChecking] = useState(true);
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [needsSetup, setNeedsSetup] = useState(false);
    const location = useLocation();

    useEffect(() => {
        checkAuth();
    }, []);

    const checkAuth = async () => {
        try {
            // 首先检查是否需要初始化设置
            const setupResponse = await fetch('/api/setup/status');
            const setupData = await setupResponse.json();

            if (setupData.needs_setup) {
                setNeedsSetup(true);
                setIsChecking(false);
                return;
            }

            // 检查是否已登录
            const token = localStorage.getItem('admin_token');
            if (!token) {
                setIsAuthenticated(false);
                setIsChecking(false);
                return;
            }

            // 验证 token
            const response = await fetch('/api/auth/me', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });

            if (response.ok) {
                setIsAuthenticated(true);
            } else {
                // Token 无效，清除
                localStorage.removeItem('admin_token');
                localStorage.removeItem('admin_user');
                setIsAuthenticated(false);
            }
        } catch (error) {
            console.error('Auth check failed:', error);
            setIsAuthenticated(false);
        } finally {
            setIsChecking(false);
        }
    };

    if (isChecking) {
        return (
            <div className="min-h-screen flex items-center justify-center">
                <div className="text-center">
                    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 mx-auto"></div>
                    <p className="mt-4 text-gray-600">验证中...</p>
                </div>
            </div>
        );
    }

    if (needsSetup) {
        return <Navigate to="/setup" state={{from: location}} replace />;
    }

    if (!isAuthenticated) {
        return <Navigate to="/login" state={{from: location}} replace />;
    }

    return <>{children}</>;
}
