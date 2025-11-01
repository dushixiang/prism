import {useEffect} from 'react';
import {QueryClientProvider} from '@tanstack/react-query';
import {queryClient} from './utils/api';
import {Dashboard} from './components/Dashboard';
import {PRICE_UNIT} from './constants/currency';

function App() {
    useEffect(() => {
        document.documentElement.setAttribute('data-price-unit', PRICE_UNIT);
    }, []);

    return (
        <QueryClientProvider client={queryClient}>
            <Dashboard/>
        </QueryClientProvider>
    );
}

export default App;
