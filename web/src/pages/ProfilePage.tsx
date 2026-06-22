import { useEffect, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import LockResetIcon from "@mui/icons-material/LockReset";
import SaveIcon from "@mui/icons-material/Save";
import { useTranslation } from "react-i18next";
import { Link as RouterLink, useLocation } from "react-router-dom";

import { useMe, useUpdateProfile } from "../api/auth";
import { useMyDCCPool } from "../api/vehicles";
import { formatDccPoolSummary } from "../components/UserDccPoolFields";

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString();
}

function ProfileField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <TableRow>
      <TableCell component="th" scope="row" sx={{ width: { sm: "40%" }, fontWeight: 500 }}>
        {label}
      </TableCell>
      <TableCell>{children}</TableCell>
    </TableRow>
  );
}

export default function ProfilePage() {
  const { t } = useTranslation(["user", "common", "role", "layout"]);
  const location = useLocation();
  const pinChanged = (location.state as { pinChanged?: boolean } | null)?.pinChanged;
  const me = useMe();
  const dccPool = useMyDCCPool();
  const updateProfile = useUpdateProfile();
  const [organizationInput, setOrganizationInput] = useState("");
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileSaved, setProfileSaved] = useState(false);

  const loading = me.isLoading || dccPool.isLoading;
  const error = me.error ?? dccPool.error;
  const user = me.data;

  useEffect(() => {
    if (user) {
      setOrganizationInput(user.organization ?? "");
    }
  }, [user]);

  const layoutLabel = user
    ? user.layoutIsSystem
      ? t("layout:system_default_label")
      : user.layoutName
    : "";

  const organizationDirty =
    user != null && organizationInput.trim() !== (user.organization ?? "");

  const saveOrganization = async () => {
    if (!user) return;
    setProfileError(null);
    setProfileSaved(false);
    try {
      await updateProfile.mutateAsync({
        organization: organizationInput.trim(),
      });
      setProfileSaved(true);
    } catch {
      setProfileError(t("common:networkError"));
    }
  };

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("user:profile.title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("user:profile.subtitle")}
          </Typography>
        </Box>

        {pinChanged && (
          <Alert severity="success">{t("user:changePin.success")}</Alert>
        )}

        {error && <Alert severity="error">{t("common:networkError")}</Alert>}

        {loading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
            <CircularProgress />
          </Box>
        ) : user ? (
          <>
            <Paper variant="outlined">
              <TableContainer>
                <Table size="small">
                  <TableBody>
                    <ProfileField label={t("user:profile.fields.login")}>
                      {user.login}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.organization")}>
                      {user.organization
                        ? user.organization
                        : t("user:profile.organizationEmpty")}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.id")}>
                      {user.id}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.role")}>
                      <Chip
                        size="small"
                        variant="outlined"
                        color={user.role === "admin" ? "primary" : "default"}
                        label={t(`role:${user.role}` as const, {
                          defaultValue: user.role,
                        })}
                      />
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.effectiveRole")}>
                      <Chip
                        size="small"
                        variant="outlined"
                        color={
                          user.effectiveRole === "admin" ? "primary" : "default"
                        }
                        label={t(`role:${user.effectiveRole}` as const, {
                          defaultValue: user.effectiveRole,
                        })}
                      />
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.isSignalman")}>
                      {user.isSignalman
                        ? t("user:profile.yes")
                        : t("user:profile.no")}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.status")}>
                      <Chip
                        size="small"
                        color={user.active ? "success" : "warning"}
                        label={
                          user.active
                            ? t("user:admin.status.active")
                            : t("user:admin.status.inactive")
                        }
                      />
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.dccPool")}>
                      {formatDccPoolSummary(dccPool.data ?? [])}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.layout")}>
                      {layoutLabel}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.sudo")}>
                      {user.sudo
                        ? t("user:profile.sudoActive", {
                            expiresAt: formatTimestamp(user.sudo.expiresAt),
                          })
                        : t("user:profile.sudoInactive")}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.createdAt")}>
                      {formatTimestamp(user.createdAt)}
                    </ProfileField>
                    <ProfileField label={t("user:profile.fields.updatedAt")}>
                      {formatTimestamp(user.updatedAt)}
                    </ProfileField>
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>

            <Paper variant="outlined" sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="subtitle1" component="h2">
                  {t("user:profile.organizationSection")}
                </Typography>
                <TextField
                  label={t("user:profile.fields.organization")}
                  value={organizationInput}
                  onChange={(e) => {
                    setOrganizationInput(e.target.value);
                    setProfileSaved(false);
                  }}
                  helperText={t("user:profile.organizationHelp")}
                  fullWidth
                  inputProps={{ maxLength: 128 }}
                />
                {profileError && <Alert severity="error">{profileError}</Alert>}
                {profileSaved && (
                  <Alert severity="success">{t("user:profile.organizationSaved")}</Alert>
                )}
                <Box>
                  <Button
                    variant="contained"
                    startIcon={<SaveIcon />}
                    onClick={saveOrganization}
                    disabled={!organizationDirty || updateProfile.isPending}
                  >
                    {t("user:profile.saveOrganization")}
                  </Button>
                </Box>
              </Stack>
            </Paper>

            <Box>
              <Button
                component={RouterLink}
                to="/account/change-pin"
                variant="contained"
                startIcon={<LockResetIcon />}
              >
                {t("user:profile.changePin")}
              </Button>
            </Box>
          </>
        ) : null}
      </Stack>
    </Container>
  );
}
