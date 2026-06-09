import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { PageShell } from '@/components/layout/PageShell'
import { ProtectedRoute } from '@/routes/ProtectedRoute'
import BrowsePage from '@/pages/BrowsePage'
import AuthPage from '@/pages/AuthPage'
import MangaDetailPage from '@/pages/MangaDetailPage'
import LibraryPage from '@/pages/LibraryPage'
import ChatPage from '@/pages/ChatPage'
import FeedPage from '@/pages/FeedPage'
import ProfilePage from '@/pages/ProfilePage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 2,
      retry: 1,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          {/* Public */}
          <Route path="/auth" element={<AuthPage />} />

          {/* Shell wraps all app routes */}
          <Route element={<PageShell />}>
            <Route path="/" element={<BrowsePage />} />
            <Route path="/manga/:id" element={<MangaDetailPage />} />

            {/* Protected */}
            <Route path="/library" element={<ProtectedRoute><LibraryPage /></ProtectedRoute>} />
            <Route path="/chat" element={<ProtectedRoute><ChatPage /></ProtectedRoute>} />
            <Route path="/chat/:room" element={<ProtectedRoute><ChatPage /></ProtectedRoute>} />
            <Route path="/feed" element={<ProtectedRoute><FeedPage /></ProtectedRoute>} />
            <Route path="/profile" element={<ProtectedRoute><ProfilePage /></ProtectedRoute>} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
