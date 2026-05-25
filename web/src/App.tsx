import {
  Navigate,
  Route,
  RouterProvider,
  createBrowserRouter,
  createRoutesFromElements,
} from "react-router-dom";

import AdminRoute from "./components/AdminRoute";
import AppShell from "./components/AppShell";
import ProtectedRoute from "./components/ProtectedRoute";
import HomePage from "./pages/HomePage";
import InterlockingPage from "./pages/InterlockingPage";
import LoginPage from "./pages/LoginPage";
import MyTrainsPage from "./pages/MyTrainsPage";
import MyVehiclesPage from "./pages/MyVehiclesPage";
import LayoutsAdminPage from "./pages/admin/LayoutsPage";
import InterlockingsAdminPage from "./pages/admin/InterlockingsPage";
import UsersAdminPage from "./pages/admin/UsersPage";

// App is the route-tree root. Layout reads top-down:
//
//   /login                       → unauthenticated only
//   <ProtectedRoute/>            → gate that bounces anon traffic to /login
//     <AppShell/>                → top app bar shared by every authenticated page
//       /                        → HomePage (dashboard)
//       /interlockings/:id       → InterlockingPage (§6.3d)
//       <AdminRoute/>            → admin-only sub-tree
//         /admin/layouts         → Layouts management (§4.1)
//         /admin/interlockings   → Interlockings catalogue (admin CRUD)
//       /*                       → fall back to /
//
// We deliberately use the **data router** (`createBrowserRouter` +
// `<RouterProvider/>`) instead of the legacy `<BrowserRouter/>` because
// `useBlocker` — used by `InterlockingPage` to prompt the user before
// leaving while still occupying — is only available in the data
// router APIs (react-router v6.4+).
const router = createBrowserRouter(
  createRoutesFromElements(
    <>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppShell />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/my/vehicles" element={<MyVehiclesPage />} />
          <Route path="/my/trains" element={<MyTrainsPage />} />
          <Route path="/interlockings/:id" element={<InterlockingPage />} />
          <Route element={<AdminRoute />}>
            <Route path="/admin/users" element={<UsersAdminPage />} />
            <Route path="/admin/layouts" element={<LayoutsAdminPage />} />
            <Route
              path="/admin/interlockings"
              element={<InterlockingsAdminPage />}
            />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Route>
    </>,
  ),
);

export default function App() {
  return <RouterProvider router={router} />;
}
