import React, { Suspense } from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { AuthProvider } from "./providers/AuthProvider";
import { RoomProvider } from "./providers/RoomProvider";
import { FriendsProvider } from "./providers/FriendsProvider";
import { RTCProvider } from "./providers/RTCProvider";
import { PlayerSyncProvider } from "./providers/PlayerSyncProvider";
import { ProtectedRoute } from "./routes/ProtectedRoute";
import { AppLayout } from "./components/layout/AppLayout";
import { Spinner } from "./components/common/Spinner";

// Route-level code splitting via React.lazy
const LoginPage = React.lazy(() =>
  import("./pages/LoginPage").then((m) => ({ default: m.LoginPage })),
);
const SignupPage = React.lazy(() =>
  import("./pages/SignupPage").then((m) => ({ default: m.SignupPage })),
);
const DashboardPage = React.lazy(() =>
  import("./pages/DashboardPage").then((m) => ({ default: m.DashboardPage })),
);
const RoomPage = React.lazy(() =>
  import("./pages/RoomPage").then((m) => ({ default: m.RoomPage })),
);
const FriendsPage = React.lazy(() =>
  import("./pages/FriendsPage").then((m) => ({ default: m.FriendsPage })),
);
const ForgotPasswordPage = React.lazy(() =>
  import("./pages/ForgotPasswordPage").then((m) => ({
    default: m.ForgotPasswordPage,
  })),
);
const ResetPasswordPage = React.lazy(() =>
  import("./pages/ResetPasswordPage").then((m) => ({
    default: m.ResetPasswordPage,
  })),
);

function GlobalLoadingFallback() {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        height: "100vh",
        width: "100vw",
        background: "var(--color-canvas)",
        flexDirection: "column",
        gap: "var(--space-3)",
      }}
    >
      <Spinner size={28} />
      <span
        style={{
          fontSize: "var(--text-compact)",
          color: "var(--color-text-secondary)",
          fontWeight: 500,
        }}
      >
        Loading…
      </span>
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <FriendsProvider>
        <RoomProvider>
          <RTCProvider>
            <PlayerSyncProvider>
              <BrowserRouter>
                <Suspense fallback={<GlobalLoadingFallback />}>
                  <Routes>
                    {/* Auth routes — no app shell */}
                    <Route path="/login" element={<LoginPage />} />
                    <Route path="/signup" element={<SignupPage />} />
                    <Route
                      path="/forgot-password"
                      element={<ForgotPasswordPage />}
                    />
                    <Route
                      path="/reset-password"
                      element={<ResetPasswordPage />}
                    />

                    {/* Dashboard — AppLayout with HomeSidebar */}
                    <Route
                      path="/"
                      element={
                        <ProtectedRoute>
                          <AppLayout variant="home">
                            <DashboardPage />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    {/* Friends — AppLayout with HomeSidebar, same shell as the lobby */}
                    <Route
                      path="/friends"
                      element={
                        <ProtectedRoute>
                          <AppLayout variant="home">
                            <FriendsPage />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    {/* Room — AppLayout with rail only (room manages its own sidebar) */}
                    <Route
                      path="/room/:roomId"
                      element={
                        <ProtectedRoute>
                          <AppLayout variant="room">
                            <RoomPage />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />
                  </Routes>
                </Suspense>
              </BrowserRouter>
            </PlayerSyncProvider>
          </RTCProvider>
        </RoomProvider>
      </FriendsProvider>
    </AuthProvider>
  );
}

export default App;
