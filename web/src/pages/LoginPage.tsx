import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Container,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import { useLogin, useMe } from "../api/auth";
import { useLoginLayouts, type LoginLayout } from "../api/layouts";
import LanguageMenu from "../components/LanguageMenu";

interface LocationState {
  from?: { pathname?: string };
}

// renderLayoutLabel resolves the user-visible label of a layout row
// from the dropdown payload (§7a.1). The bootstrap system row stores
// a stable Name ("default") that is NEVER rendered directly — instead
// the UI substitutes the `layout:system_default_label` i18n key so the
// switcher reads "Domyślna (warsztat)" / "Default (workshop)" out of
// the box.
function renderLayoutLabel(layout: LoginLayout, systemLabel: string): string {
  return layout.isSystem ? systemLabel : layout.name;
}

// LoginPage renders the credentials form. The handler dispatches
// useLogin (which writes the resulting user into the meQuery cache);
// the Navigate below picks the change up on the next render and
// bounces the user to wherever they were originally headed.
export default function LoginPage() {
  const [login, setLogin] = useState("");
  const [pin, setPin] = useState("");
  // `layoutId === 0` means "not yet picked"; the effect below selects
  // the system layout as soon as the dropdown payload arrives, so the
  // value is 0 only during the brief loading window.
  const [layoutId, setLayoutId] = useState<number>(0);

  const loginMut = useLogin();
  const me = useMe();
  const layouts = useLoginLayouts();
  const location = useLocation();

  // useTranslation accepts an array of namespaces — the first one is
  // the default for un-prefixed lookups, the rest must be prefixed
  // explicitly (`t("errors:invalid_credentials")`).
  const { t } = useTranslation(["auth", "errors", "common", "layout"]);

  const systemLabel = t("layout:system_default_label");

  // Pre-select the system layout on first paint, matching §7a.1:
  // "It is also the dropdown's default pre-selected entry on first
  // paint, so a user who never touches the selector simply lands in
  // the system layout."
  useEffect(() => {
    if (layoutId !== 0) return;
    if (!layouts.data || layouts.data.length === 0) return;
    const sys = layouts.data.find((l) => l.isSystem);
    setLayoutId(sys?.id ?? layouts.data[0].id);
  }, [layouts.data, layoutId]);

  // Memoise the dropdown options so MUI's Select doesn't re-render
  // every keystroke in the login/PIN fields.
  const layoutOptions = useMemo(() => {
    if (!layouts.data) return [];
    return layouts.data.map((l) => ({
      id: l.id,
      label: renderLayoutLabel(l, systemLabel),
    }));
  }, [layouts.data, systemLabel]);

  // Already authenticated → straight to the protected app.
  if (me.data) {
    const dest =
      (location.state as LocationState | undefined)?.from?.pathname ?? "/";
    return <Navigate to={dest} replace />;
  }

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (layoutId === 0) return;
    loginMut.mutate({ login: login.trim(), pin, layoutId });
  };

  // ApiError.code is machine-readable on purpose — we map it 1:1 to a
  // key in the `errors` namespace. Unknown codes fall through to a
  // generic message that still surfaces the raw code so a support
  // request can be filed without losing context.
  const errMessage = (() => {
    if (!loginMut.error) return null;
    if (loginMut.error instanceof ApiError) {
      const code = loginMut.error.code;
      const key = `errors:${code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code });
    }
    return t("errors:network");
  })();

  const layoutsLoading = layouts.isLoading;
  const layoutsError = layouts.isError;
  // Buttons stay disabled until either the layout list is in flight
  // or we have no row to send. Once a row is picked the submit is
  // free to fire even if a refetch is happening in the background.
  const submitDisabled =
    loginMut.isPending || layoutsLoading || layoutsError || layoutId === 0;

  return (
    <Box
      sx={{
        position: "relative",
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: "background.default",
        p: 2,
      }}
    >
      <Box
        sx={{
          position: "absolute",
          top: { xs: 8, sm: 16 },
          right: { xs: 8, sm: 16 },
          zIndex: 1,
        }}
      >
        <LanguageMenu />
      </Box>

      <Container maxWidth="xs" disableGutters>
        <Card elevation={3}>
          <CardContent sx={{ p: { xs: 3, sm: 4 } }}>
            <Stack spacing={1} alignItems="center" sx={{ mb: 3 }}>
              <Typography variant="h4" component="h1" sx={{ fontWeight: 600 }}>
                {t("login.title")}
              </Typography>
              <Typography variant="body2" color="text.secondary" textAlign="center">
                {t("login.subtitle")}
              </Typography>
            </Stack>

            <form onSubmit={onSubmit} noValidate>
              <Stack spacing={2}>
                <TextField
                  label={t("login.fields.login")}
                  value={login}
                  onChange={(e) => setLogin(e.target.value)}
                  autoComplete="username"
                  autoFocus
                  fullWidth
                  required
                />
                <TextField
                  label={t("login.fields.pin")}
                  type="password"
                  value={pin}
                  onChange={(e) => setPin(e.target.value)}
                  autoComplete="current-password"
                  inputProps={{ inputMode: "text" }}
                  fullWidth
                  required
                />

                {/* Layout picker — §7a.1. Disabled until the public
                    dropdown payload arrives so the user can never
                    accidentally submit `layoutId === 0`. Errors loading
                    the list surface as an Alert below; the submit
                    button is then disabled too. */}
                <TextField
                  select
                  label={t("layout:loginPicker.label")}
                  value={layoutId === 0 ? "" : String(layoutId)}
                  onChange={(e) => setLayoutId(Number(e.target.value))}
                  disabled={layoutsLoading || layoutsError || layoutOptions.length === 0}
                  fullWidth
                  required
                  helperText={
                    layoutsLoading
                      ? t("layout:loginPicker.loading")
                      : layoutsError
                        ? t("layout:loginPicker.loadError")
                        : layoutOptions.length === 0
                          ? t("layout:loginPicker.empty")
                          : undefined
                  }
                  InputProps={
                    layoutsLoading
                      ? {
                          endAdornment: (
                            <CircularProgress size={18} sx={{ mr: 3 }} />
                          ),
                        }
                      : undefined
                  }
                >
                  {layoutOptions.map((opt) => (
                    <MenuItem key={opt.id} value={String(opt.id)}>
                      {opt.label}
                    </MenuItem>
                  ))}
                </TextField>

                {errMessage && <Alert severity="error">{errMessage}</Alert>}

                <Button
                  type="submit"
                  variant="contained"
                  size="large"
                  disabled={submitDisabled}
                  fullWidth
                >
                  {loginMut.isPending ? t("login.submitting") : t("login.submit")}
                </Button>
              </Stack>
            </form>
          </CardContent>
        </Card>
      </Container>
    </Box>
  );
}
