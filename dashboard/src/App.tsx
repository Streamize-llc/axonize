import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/layout/Layout';
import { Overview } from './pages/Overview';
import { Traces } from './pages/Traces';
import { TraceDetailPage } from './pages/TraceDetail';
import { GPUs } from './pages/GPUs';
import { GPUDetailPage } from './pages/GPUDetail';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      retry: 1,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Overview />} />
            <Route path="/traces" element={<Traces />} />
            <Route path="/traces/:traceId" element={<TraceDetailPage />} />
            <Route path="/gpus" element={<GPUs />} />
            <Route path="/gpus/:uuid" element={<GPUDetailPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
