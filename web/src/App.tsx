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
import WakeLockKeeper from "./components/WakeLockKeeper";
import HomePage from "./pages/HomePage";
import InterlockingPage from "./pages/InterlockingPage";
import LoginPage from "./pages/LoginPage";
import AvailableVehiclesPage from "./pages/AvailableVehiclesPage";
import AvailableTrainsPage from "./pages/AvailableTrainsPage";
import VehicleFunctionsPage from "./pages/VehicleFunctionsPage";
import VehicleTemplatesPage from "./pages/VehicleTemplatesPage";
import TemplateFunctionsPage from "./pages/TemplateFunctionsPage";
import RentalsPage from "./pages/RentalsPage";
import ThrottlePage from "./pages/ThrottlePage";
import AuditLogPage from "./pages/AuditLogPage";
import ProfilePage from "./pages/ProfilePage";
import ChangePinPage from "./pages/ChangePinPage";
import VersionPage from "./pages/VersionPage";
import LayoutsAdminPage from "./pages/admin/LayoutsPage";
import InterlockingsAdminPage from "./pages/admin/InterlockingsPage";
import CommandStationsAdminPage from "./pages/admin/CommandStationsPage";
import ConnectionWizardAdminPage from "./pages/admin/ConnectionWizardPage";
import SlotsDiagnosticsAdminPage from "./pages/admin/SlotsDiagnosticsPage";
import DiagnosticsAdminPage from "./pages/admin/DiagnosticsPage";
import SystemAdminPage from "./pages/admin/SystemPage";
import AdminRentalsPage from "./pages/admin/AdminRentalsPage";
import UsersAdminPage from "./pages/admin/UsersPage";
import RemotesPage from "./pages/remotes/RemotesPage";
import DccPoolsPage from "./pages/DccPoolsPage";

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
          <Route path="/my/vehicles" element={<AvailableVehiclesPage />} />
          <Route path="/my/trains" element={<AvailableTrainsPage />} />
          <Route path="/fleet/vehicles" element={<Navigate to="/my/vehicles" replace />} />
          <Route path="/fleet/trains" element={<Navigate to="/my/trains" replace />} />
          <Route path="/my/vehicles/:vehicleId/functions" element={<VehicleFunctionsPage />} />
          <Route path="/vehicle-templates" element={<VehicleTemplatesPage />} />
          <Route
            path="/vehicle-templates/:templateId/functions"
            element={<TemplateFunctionsPage />}
          />
          <Route path="/rentals" element={<RentalsPage />} />
          <Route path="/dcc-pools" element={<DccPoolsPage />} />
          <Route path="/throttle" element={<ThrottlePage />} />
          <Route path="/remotes" element={<RemotesPage />} />
          <Route path="/remotes/z21" element={<Navigate to="/remotes" replace />} />
          <Route path="/remotes/withrottle" element={<Navigate to="/remotes" replace />} />
          <Route path="/interlockings/:id" element={<InterlockingPage />} />
          <Route path="/audit-log" element={<AuditLogPage />} />
          <Route path="/account/profile" element={<ProfilePage />} />
          <Route path="/account/change-pin" element={<ChangePinPage />} />
          <Route path="/account/version" element={<VersionPage />} />
          <Route element={<AdminRoute />}>
            <Route path="/admin/users" element={<UsersAdminPage />} />
            <Route path="/admin/layouts" element={<LayoutsAdminPage />} />
            <Route
              path="/admin/interlockings"
              element={<InterlockingsAdminPage />}
            />
            <Route
              path="/admin/command-stations"
              element={<CommandStationsAdminPage />}
            />
            <Route
              path="/admin/command-stations/wizard"
              element={<ConnectionWizardAdminPage />}
            />
            <Route
              path="/admin/dcc-bus/slots"
              element={<SlotsDiagnosticsAdminPage />}
            />
            <Route path="/admin/logs" element={<DiagnosticsAdminPage />} />
            <Route path="/admin/system" element={<SystemAdminPage />} />
            <Route path="/admin/rentals" element={<AdminRentalsPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Route>
    </>,
  ),
);

export default function App() {
  return (
    <>
      <WakeLockKeeper />
      <RouterProvider router={router} />
    </>
  );
}
