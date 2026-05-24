import { Box, Container, Paper, Stack, Typography } from "@mui/material";
import { Trans, useTranslation } from "react-i18next";

import { useMe } from "../api/auth";

// HomePage is the placeholder landing screen for the bootstrap
// milestone. It exists so the rest of the auth plumbing
// (ProtectedRoute → AppShell → child) has something to render after
// a successful login. Real content (party picker, vehicle list, …)
// will replace this in M3+.
export default function HomePage() {
  const me = useMe().data;
  const { t } = useTranslation("home");

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {me
              ? t("greeting", { login: me.login })
              : t("greetingAnon")}
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {/* Trans renders inline elements (<code/>) sourced from
                the catalogue. Components are passed by name; the
                index matches the i18nKey markup, so `<code>...</code>`
                in JSON maps to <Box component="code" .../>. */}
            <Trans
              i18nKey="intro"
              t={t}
              components={{
                code: (
                  <Box
                    component="code"
                    sx={{
                      px: 0.5,
                      py: 0.25,
                      borderRadius: 0.5,
                      bgcolor: "action.hover",
                      fontFamily: "monospace",
                    }}
                  />
                ),
              }}
            />
          </Typography>
        </Box>

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }}>
          <Typography variant="overline" color="text.secondary">
            {t("milestoneOverline")}
          </Typography>
          <Typography variant="h6" gutterBottom>
            {t("milestoneTitle")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("milestoneBody")}
          </Typography>
        </Paper>
      </Stack>
    </Container>
  );
}
