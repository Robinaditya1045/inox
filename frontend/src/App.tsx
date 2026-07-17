import React, { Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './providers/AuthProvider';
import { RoomProvider } from './providers/RoomProvider';
import { WSProvider } from './providers/WSProvider';
import { RTCProvider } from './providers/RTCProvider';
import { ProtectedRoute } from './routes/ProtectedRoute';
import { AppLayout } from './components/layout/AppLayout';
import { Spinner } from './components/common/Spinner';

// Route-level code splitting via React.lazy
const LoginPage = React.lazy(() => import('./pages/LoginPage').then((m) => ({ default: m.LoginPage })));
const SignupPage = React.lazy(() => import('./pages/SignupPage').then((m) => ({ default: m.SignupPage })));
const DashboardPage = React.lazy(() => import('./pages/DashboardPage').then((m) => ({ default: m.DashboardPage })));
const RoomPage = React.lazy(() => import('./pages/RoomPage').then((m) => ({ default: m.RoomPage })));

// Full-screen layout for the room — no global sidebar, the room has its own channel sidebar
function RoomLayout({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        display: 'flex',
        height: '100vh',
        width: '100vw',
        background: 'var(--color-bg-obsidian)',
        color: 'var(--color-text-primary)',
        overflow: 'hidden',
      }}
    >
      {children}
    </div>
  );
}

function GlobalLoadingFallback() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', width: '100vw', background: 'var(--color-bg-obsidian)', flexDirection: 'column', gap: '12px' }}>
      <Spinner size={32} color="var(--color-accent-purple)" />
      <span style={{ fontSize: '0.9rem', color: 'var(--color-text-secondary)', fontWeight: 500 }}>Loading Inox WatchParty...</span>
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <RoomProvider>
        <WSProvider>
          <RTCProvider>
            <BrowserRouter>
              <Suspense fallback={<GlobalLoadingFallback />}>
                <Routes>
                  <Route path="/login" element={<LoginPage />} />
                  <Route path="/signup" element={<SignupPage />} />
                  <Route
                    path="/"
                    element={
                      <ProtectedRoute>
                        <AppLayout>
                          <DashboardPage />
                        </AppLayout>
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/room/:roomId"
                    element={
                      <ProtectedRoute>
                        <RoomLayout>
                          <RoomPage />
                        </RoomLayout>
                      </ProtectedRoute>
                    }
                  />
                </Routes>
              </Suspense>
            </BrowserRouter>
          </RTCProvider>
        </WSProvider>
      </RoomProvider>
    </AuthProvider>
  );
}

export default App;
