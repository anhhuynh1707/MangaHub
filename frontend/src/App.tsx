import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { PageShell } from '@/components/layout/PageShell'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { ProtectedRoute } from '@/routes/ProtectedRoute'
import { useUIStore } from '@/store/uiStore'
import BrowsePage from '@/pages/BrowsePage'
import AuthPage from '@/pages/AuthPage'
import MangaDetailPage from '@/pages/MangaDetailPage'
import LibraryPage from '@/pages/LibraryPage'
import ChatPage from '@/pages/ChatPage'
import FeedPage from '@/pages/FeedPage'
import ProfilePage from '@/pages/ProfilePage'
import ChangePasswordPage from '@/pages/ChangePasswordPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 2,
      retry: 1,
    },
  },
})

export default function App() {
  const theme = useUIStore((s) => s.theme)

  return (
    <QueryClientProvider client={queryClient}>
      {/* Global toast host — themed to match the app, errors in rich colors */}
      <Toaster theme={theme} richColors closeButton position="top-right" />
      <BrowserRouter>
        <Routes>
          {/* Public */}
          <Route path="/auth" element={<ErrorBoundary label="Sign in"><AuthPage /></ErrorBoundary>} />

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
            <Route path="/change-password" element={<ProtectedRoute><ChangePasswordPage /></ProtectedRoute>} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
