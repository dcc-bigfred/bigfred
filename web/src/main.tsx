import React from "react";
import ReactDOM from "react-dom/client";
import { CssBaseline, ThemeProvider } from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";

import "@fontsource/roboto/400.css";
import "@fontsource/roboto/500.css";
import "@fontsource/roboto/700.css";

import App from "./App";
import { theme } from "./theme";
import i18n from "./i18n";
import "./i18n/types";

// One QueryClient per page load (created at module init); the rest
// of the app retrieves it through useQuery / useMutation hooks.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

// Per §7c.8, query keys that contain locale-sensitive copy must be
// invalidated when the user switches language. At the bootstrap
// milestone NO such queries exist yet (every payload on the wire is
// either a stable code or user-entered text rendered verbatim), but
// wiring the subscription here means future locale-aware lists
// (audit-log, activity-feed) automatically refetch without revisiting
// this file.
i18n.on("languageChanged", () => {
  queryClient.invalidateQueries({ queryKey: ["audit-log"] });
  queryClient.invalidateQueries({ queryKey: ["activity-feed"] });
});

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("#root element not found in index.html");
}

// Provider order matters (§7c.5):
//   I18nextProvider  ← outermost: every error toast / dialog below
//                                 reads the active locale.
//   QueryClient      ← needs i18n above so onError handlers can use
//                                 t() for user-facing messages.
//   ThemeProvider    ← MUI's LocalizationProvider (date pickers, M3+)
//                                 will read the locale from i18n.
//   App              ← routes, pages, etc.
ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          <App />
        </ThemeProvider>
      </QueryClientProvider>
    </I18nextProvider>
  </React.StrictMode>,
);
