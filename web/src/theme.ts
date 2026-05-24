import { createTheme } from "@mui/material/styles";

// Single ThemeProvider configuration referenced from main.tsx.
// Kept intentionally small at the bootstrap milestone — colours and
// typography overrides will grow as the locomotive-control screens
// land in later milestones.
export const theme = createTheme({
  palette: {
    mode: "light",
    primary: { main: "#0d47a1" },
    secondary: { main: "#ff6f00" },
    background: { default: "#f4f6f8" },
  },
  typography: {
    fontFamily:
      'Roboto, system-ui, -apple-system, "Segoe UI", "Helvetica Neue", Arial, sans-serif',
  },
  shape: { borderRadius: 8 },
});
