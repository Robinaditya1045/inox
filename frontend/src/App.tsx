import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './providers/AuthProvider';
import { RoomProvider } from './providers/RoomProvider';
import { WSProvider } from './providers/WSProvider';
import { RTCProvider } from './providers/RTCProvider';
import { ProtectedRoute } from './routes/ProtectedRoute';
import { LoginPage } from './pages/LoginPage';
import { SignupPage } from './pages/SignupPage';
import { DashboardPage } from './pages/DashboardPage';
import { RoomPage } from './pages/RoomPage';
import { AppLayout } from './components/layout/AppLayout';

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

function App() {
  return (
    <AuthProvider>
      <RoomProvider>
        <WSProvider>
          <RTCProvider>
            <BrowserRouter>
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
            </BrowserRouter>
          </RTCProvider>
        </WSProvider>
      </RoomProvider>
    </AuthProvider>
  );
}

export default App;
