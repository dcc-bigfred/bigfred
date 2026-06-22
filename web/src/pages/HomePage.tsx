import SettingsIcon from "@mui/icons-material/Settings";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useMe } from "../api/auth";
import { useDashboardInterlockings } from "../api/interlockings";
import { useLayoutPresence } from "../api/presence";
import PromoteSignalmanButton from "../components/PromoteSignalmanButton";
import DemoteSignalmanButton from "../components/DemoteSignalmanButton";
import RosterSection from "../components/RosterSection";
import { getUserName } from "../utils/getUserName";

export default function HomePage() {
  const me = useMe().data;
  const layoutId = me?.layoutId ?? null;
  const { t } = useTranslation(["home", "role", "layout", "interlocking"]);
  const navigate = useNavigate();

  const presence = useLayoutPresence(layoutId);
  const interlockings = useDashboardInterlockings();

  const loading = presence.isLoading || interlockings.isLoading;
  const error = presence.error ?? interlockings.error;

  return (
    <Container maxWidth="lg" sx={{ py: { xs: 3, sm: 4 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {me
              ? t("home:title", {
                  layoutName: me.layoutIsSystem
                    ? t("layout:system_default_label")
                    : me.layoutName,
                })
              : t("home:greetingAnon")}
          </Typography>
          {me && (
            <Typography variant="body1" color="text.secondary">
              {t("home:subtitle", {
                login: getUserName(me),
                role: t(`role:${me.effectiveRole}` as const, {
                  defaultValue: me.effectiveRole,
                }),
              })}
            </Typography>
          )}
        </Box>

        {error && (
          <Alert severity="error">{t("home:loadError")}</Alert>
        )}

        {loading ? (
          <Stack alignItems="center" py={6}>
            <CircularProgress />
          </Stack>
        ) : (
          <>
            <Paper variant="outlined">
              <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}>
                <Typography variant="h6">{t("home:onlineUsers.title")}</Typography>
              </Box>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>{t("home:onlineUsers.columns.login")}</TableCell>
                      <TableCell>{t("home:onlineUsers.columns.role")}</TableCell>
                      <TableCell>{t("home:onlineUsers.columns.interlocking")}</TableCell>
                      <TableCell align="right" width={96}>
                        {t("home:onlineUsers.columns.actions")}
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {(presence.data ?? []).length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={4} align="center" sx={{ py: 3, color: "text.secondary" }}>
                          {t("home:onlineUsers.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      (presence.data ?? []).map((user) => (
                        <TableRow key={user.userId}>
                          <TableCell>{getUserName(user)}</TableCell>
                          <TableCell>
                            {t(`role:${user.role}` as const, { defaultValue: user.role })}
                          </TableCell>
                          <TableCell>
                            {user.occupiedInterlocking?.name ?? t("home:onlineUsers.noInterlocking")}
                          </TableCell>
                          <TableCell align="right">
                            {layoutId != null && (
                              <Stack
                                direction="row"
                                spacing={0.5}
                                justifyContent="flex-end"
                              >
                                <PromoteSignalmanButton layoutId={layoutId} user={user} />
                                <DemoteSignalmanButton layoutId={layoutId} user={user} />
                              </Stack>
                            )}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>

            <Paper variant="outlined">
              <Box
                sx={{
                  px: 2,
                  py: 1.5,
                  borderBottom: 1,
                  borderColor: "divider",
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                }}
              >
                <Typography variant="h6" sx={{ flexGrow: 1 }}>
                  {t("home:interlockings.title")}
                </Typography>
                {me?.effectiveRole === "admin" &&
                  layoutId != null &&
                  layoutId > 0 && (
                    <Button
                      variant="outlined"
                      size="small"
                      startIcon={<SettingsIcon />}
                      onClick={() => navigate(`/admin/layouts?edit=${layoutId}`)}
                    >
                      {t("home:editLayoutSettings")}
                    </Button>
                  )}
              </Box>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>{t("home:interlockings.columns.name")}</TableCell>
                      <TableCell>{t("home:interlockings.columns.location")}</TableCell>
                      <TableCell>{t("home:interlockings.columns.occupant")}</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {(interlockings.data ?? []).length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={3} align="center" sx={{ py: 3, color: "text.secondary" }}>
                          {t("home:interlockings.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      (interlockings.data ?? []).map((row) => (
                        <TableRow
                          key={row.id}
                          hover
                          onClick={() => navigate(`/interlockings/${row.id}`)}
                          sx={{ cursor: "pointer" }}
                        >
                          <TableCell>{row.name}</TableCell>
                          <TableCell>{row.location || t("home:interlockings.noLocation")}</TableCell>
                          <TableCell>
                            {row.occupant
                              ? getUserName(row.occupant)
                              : t("home:interlockings.vacant")}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>

            {layoutId != null && layoutId > 0 && (
              <RosterSection layoutId={layoutId} />
            )}
          </>
        )}
      </Stack>
    </Container>
  );
}
