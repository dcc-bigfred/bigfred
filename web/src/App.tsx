import { Navigate, Route, BrowserRouter, Routes } from "react-router-dom";
import AdminRoute from "./components/AdminRoute";
import AppShell from "./components/AppShell";
import ProtectedRoute from "./components/ProtectedRoute";
import HomePage from "./pages/HomePage";
import LoginPage from "./pages/LoginPage";
import LayoutsAdminPage from "./pages/admin/LayoutsPage";
import InterlockingsAdminPage from "./pages/admin/InterlockingsPage";

// App is the route-tree root. Layout reads top-down:
//
//   /login                  → unauthenticated only
//   <ProtectedRoute/>       → gate that bounces anon traffic to /login
//     <AppShell/>           → top app bar shared by every authenticated page
//       /                   → HomePage (placeholder for the bootstrap)
//       <AdminRoute/>       → admin-only sub-tree (UI shortcut; the
//                             backend enforces RequireRole(admin))
//         /admin/layouts    → Layouts management (§4.1)
//         /admin/interlockings → Interlockings catalogue (admin CRUD)
//       /*                  → fall back to /
export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<AppShell />}>
            <Route path="/" element={<HomePage />} />
            <Route element={<AdminRoute />}>
              <Route path="/admin/layouts" element={<LayoutsAdminPage />} />
              <Route
                path="/admin/interlockings"
                element={<InterlockingsAdminPage />}
              />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
