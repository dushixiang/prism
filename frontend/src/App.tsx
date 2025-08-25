import {BrowserRouter as Router, Navigate, Route, Routes} from 'react-router-dom';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {Layout} from './components/Layout';
import {Dashboard} from './components/Dashboard';
import {MarketAnalysis} from './components/MarketAnalysis';
import {NewsPage} from './components/News';
import {Login} from './components/Login';
import {AIAnalysis} from './components/ai/AIAnalysis';
import {ProfilePage} from './components/account/Profile';
import {ChangePasswordPage} from './components/account/ChangePassword';

// 创建 QueryClient 实例
const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            retry: 2,
            refetchOnWindowFocus: false,
            staleTime: 30000, // 30秒缓存
        },
    },
});

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <Router>
                <Layout>
                    <Routes>
                        <Route path="/" element={<Navigate to="/dashboard" replace/>}/>
                        <Route path="/login" element={<Login/>}/>
                        <Route path="/dashboard" element={<Dashboard/>}/>
                        <Route path="/market-analysis" element={<MarketAnalysis/>}/>
                        <Route path="/ai-analysis" element={<AIAnalysis/>}/>
                        <Route path="/news" element={<NewsPage/>}/>
                        <Route path="/account/profile" element={<ProfilePage/>}/>
                        <Route path="/account/change-password" element={<ChangePasswordPage/>}/>
                        {/* 404页面 */}
                        <Route path="*" element={<Navigate to="/dashboard" replace/>}/>
                    </Routes>
                </Layout>
            </Router>
        </QueryClientProvider>
    );
}

export default App;