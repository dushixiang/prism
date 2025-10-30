import {QueryClientProvider} from '@tanstack/react-query';
import {queryClient} from './utils/api';
import {Dashboard} from './components/Dashboard';

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <Dashboard/>
        </QueryClientProvider>
    );
}

export default App;
