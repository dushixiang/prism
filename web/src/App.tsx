import {useEffect} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router-dom';
import {QueryClientProvider} from '@tanstack/react-query';
import {queryClient} from './utils/api';
import {Dashboard} from './components/Dashboard';
import {AdminLayout} from './components/admin/AdminLayout';
import {Setup} from './components/Setup';
import {Login} from './components/Login';
import {AdminDashboard} from './components/admin/AdminDashboard';
import {SystemControl} from './components/admin/SystemControl';
import {ConfigManagement} from './components/admin/ConfigManagement';
import {UserManagement} from './components/admin/UserManagement';
import {ProtectedRoute} from './components/admin/ProtectedRoute';
import {PRICE_UNIT} from './constants/currency';

function App() {
    useEffect(() => {
        document.documentElement.setAttribute('data-price-unit', PRICE_UNIT);
    }, []);

    return (
        <QueryClientProvider client={queryClient}>
            <BrowserRouter>
                <Routes>
                    {/* 公开路由 */}
                    <Route path="/" element={<Dashboard/>}/>

                    {/* 设置和登录路由 */}
                    <Route path="/setup" element={<Setup/>}/>
                    <Route path="/login" element={<Login/>}/>

                    {/* 管理后台路由（需要认证） */}
                    <Route path="/admin" element={
                        <ProtectedRoute>
                            <AdminLayout/>
                        </ProtectedRoute>
                    }>
                        <Route index element={<AdminDashboard/>}/>
                        <Route path="control" element={<SystemControl/>}/>
                        <Route path="config" element={<ConfigManagement/>}/>
                        <Route path="users" element={<UserManagement/>}/>
                    </Route>

                    {/* 404 */}
                    <Route path="*" element={<Navigate to="/" replace/>}/>
                </Routes>
            </BrowserRouter>
        </QueryClientProvider>
    );
}

export default App;
