import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Container,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import { useLogin, useMe } from "../api/auth";
import LanguageMenu from "../components/LanguageMenu";

interface LocationState {
  from?: { pathname?: string };
}

// LoginPage renders the credentials form. The handler dispatches
// useLogin (which writes the resulting user into the meQuery cache);
// the Navigate below picks the change up on the next render and
// bounces the user to wherever they were originally headed.
export default function LoginPage() {
  const [login, setLogin] = useState("");
  const [pin, setPin] = useState("");
  const loginMut = useLogin();
  const me = useMe();
  const location = useLocation();

  // useTranslation accepts an array of namespaces — the first one is
  // the default for un-prefixed lookups, the rest must be prefixed
  // explicitly (`t("errors:invalid_credentials")`).
  const { t } = useTranslation(["auth", "errors", "common"]);

  // Already authenticated → straight to the protected app.
  if (me.data) {
    const dest =
      (location.state as LocationState | undefined)?.from?.pathname ?? "/";
    return <Navigate to={dest} replace />;
  }

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    loginMut.mutate({ login: login.trim(), pin });
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
      // `i18n.exists` would be cleaner but requires an extra import;
      // `i18next.exists(key)` is also valid. Here we just check
      // `t(...) !== key` (i18next returns the key when missing,
      // because returnNull=false in init).
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code });
    }
    return t("errors:network");
  })();

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
      {/* Language switcher floats above the centred card so the
          user can change language WITHOUT a session. AppShell isn't
          mounted on this route, hence the absolute placement instead
          of the usual AppBar slot. `color="default"` (set inside the
          component via theme inheritance) renders dark text on the
          light background. */}
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
                  // The architecture spec (§7a.1) calls this field a
                  // numeric PIN; the bootstrap presents it as a
                  // password to keep UX simple. The backend stores
                  // either form the same way (argon2id).
                  inputProps={{ inputMode: "text" }}
                  fullWidth
                  required
                />

                {errMessage && <Alert severity="error">{errMessage}</Alert>}

                <Button
                  type="submit"
                  variant="contained"
                  size="large"
                  disabled={loginMut.isPending}
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
