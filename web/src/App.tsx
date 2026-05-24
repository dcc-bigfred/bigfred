import { Navigate, Route, BrowserRouter, Routes } from "react-router-dom";
import AppShell from "./components/AppShell";
import ProtectedRoute from "./components/ProtectedRoute";
import HomePage from "./pages/HomePage";
import LoginPage from "./pages/LoginPage";

// App is the route-tree root. Layout reads top-down:
//
//   /login            → unauthenticated only
//   <ProtectedRoute/> → gate that bounces anon traffic to /login
//     <AppShell/>     → top app bar shared by every authenticated page
//       /            → HomePage (placeholder for the bootstrap)
//       /*           → fall back to /
export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<AppShell />}>
            <Route path="/" element={<HomePage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
