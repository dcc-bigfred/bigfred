import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import EditIcon from "@mui/icons-material/Edit";
import DeleteIcon from "@mui/icons-material/Delete";
import LockIcon from "@mui/icons-material/Lock";
import LockOpenIcon from "@mui/icons-material/LockOpen";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import { useMe, type Role } from "../../api/auth";
import {
  useDeleteUser,
  useSetUserActive,
  useUsers,
  type User,
} from "../../api/users";
import UserEditorDialog, {
  type UserEditorDialogMode,
} from "../../components/UserEditorDialog";
import { formatDccPoolSummary } from "../../components/UserDccPoolFields";
import { getUserName } from "../../utils/getUserName";

// UsersPage is the admin-only management screen for user accounts
// (§4.1 / §7a.5). Five operations are supported:
//
//   * create   – open the dialog with empty fields, picks role + PIN
//   * edit     – rename, change role, rotate PIN (PIN field stays empty
//                unless the admin types into it)
//   * activate / deactivate – soft-disable the account; the actor's
//                own row is locked out of this operation to avoid
//                self-lockout, mirroring the backend security policy
//   * delete   – hard remove; the backend refuses when the user still
//                owns vehicles or trains, which the dialog surfaces
//                through the standard error pipeline
//
// Mutations flow through TanStack hooks that invalidate the `users`
// query, so a successful action propagates to every visible row in a
// single re-render.
export default function UsersPage() {
  const { t } = useTranslation(["user", "common", "errors", "role"]);
  const me = useMe().data;
  const list = useUsers();
  const remove = useDeleteUser();
  const setActive = useSetUserActive();

  type ConfirmState =
    | { kind: "delete"; target: User }
    | { kind: "activate"; target: User; active: boolean }
    | null;

  const [editor, setEditor] = useState<UserEditorDialogMode | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const closeConfirm = () => {
    setConfirm(null);
    setActionError(null);
    remove.reset();
    setActive.reset();
  };

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const openCreate = () => setEditor({ kind: "create" });
  const openEdit = (target: User) => setEditor({ kind: "edit", target });

  const openDelete = (target: User) => {
    setConfirm({ kind: "delete", target });
    setActionError(null);
  };

  const openSetActive = (target: User, active: boolean) => {
    setConfirm({ kind: "activate", target, active });
    setActionError(null);
  };

  const submitConfirm = async () => {
    if (!confirm) return;
    try {
      if (confirm.kind === "delete") {
        await remove.mutateAsync(confirm.target.id);
      } else {
        await setActive.mutateAsync({
          id: confirm.target.id,
          active: confirm.active,
        });
      }
      closeConfirm();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const submitting = remove.isPending || setActive.isPending;

  const renderRole = (role: Role) =>
    t(`role:${role}` as const, { defaultValue: role });

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Stack
          direction={{ xs: "column", sm: "row" }}
          alignItems={{ xs: "flex-start", sm: "center" }}
          justifyContent="space-between"
          spacing={2}
        >
          <Box>
            <Typography variant="h4" component="h1" gutterBottom>
              {t("user:admin.title")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t("user:admin.subtitle")}
            </Typography>
          </Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openCreate}
            disabled={submitting}
          >
            {t("user:admin.actions.create")}
          </Button>
        </Stack>

        {list.isError && (
          <Alert severity="error">{translateError(list.error)}</Alert>
        )}

        <Paper variant="outlined">
          {list.isLoading ? (
            <Box sx={{ p: 4, display: "flex", justifyContent: "center" }}>
              <CircularProgress />
            </Box>
          ) : rows.length === 0 ? (
            <Box sx={{ p: 4 }}>
              <Typography variant="body2" color="text.secondary">
                {t("user:admin.empty")}
              </Typography>
            </Box>
          ) : (
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>{t("user:admin.columns.login")}</TableCell>
                    <TableCell>{t("user:admin.columns.role")}</TableCell>
                    <TableCell>{t("user:admin.columns.dccPool")}</TableCell>
                    <TableCell>{t("user:admin.columns.status")}</TableCell>
                    <TableCell align="right">
                      {t("user:admin.columns.actions")}
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((u) => {
                    const isSelf = me?.id === u.id;
                    return (
                      <TableRow key={u.id} hover>
                        <TableCell>
                          <Stack direction="row" spacing={1} alignItems="center">
                            <Typography variant="body2" fontWeight={500}>
                              {getUserName(u)}
                            </Typography>
                            {isSelf && (
                              <Chip
                                size="small"
                                variant="outlined"
                                label={t("user:admin.youBadge")}
                              />
                            )}
                          </Stack>
                        </TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            variant="outlined"
                            color={u.role === "admin" ? "primary" : "default"}
                            label={renderRole(u.role)}
                          />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" color="text.secondary">
                            {formatDccPoolSummary(u.dccPool ?? [])}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            color={u.active ? "success" : "warning"}
                            label={
                              u.active
                                ? t("user:admin.status.active")
                                : t("user:admin.status.inactive")
                            }
                          />
                        </TableCell>
                        <TableCell align="right">
                          <Stack
                            direction="row"
                            spacing={0.5}
                            justifyContent="flex-end"
                          >
                            <Tooltip title={t("user:admin.actions.edit")}>
                              <span>
                                <IconButton
                                  size="small"
                                  onClick={() => openEdit(u)}
                                  disabled={submitting}
                                  aria-label={t("user:admin.actions.edit")}
                                >
                                  <EditIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>

                            {u.active ? (
                              <Tooltip
                                title={
                                  isSelf
                                    ? t("errors:cannot_deactivate_self")
                                    : t("user:admin.actions.deactivate")
                                }
                              >
                                <span>
                                  <IconButton
                                    size="small"
                                    onClick={() => openSetActive(u, false)}
                                    disabled={isSelf || submitting}
                                    aria-label={t(
                                      "user:admin.actions.deactivate",
                                    )}
                                  >
                                    <LockIcon fontSize="small" />
                                  </IconButton>
                                </span>
                              </Tooltip>
                            ) : (
                              <Tooltip title={t("user:admin.actions.activate")}>
                                <span>
                                  <IconButton
                                    size="small"
                                    onClick={() => openSetActive(u, true)}
                                    disabled={submitting}
                                    aria-label={t(
                                      "user:admin.actions.activate",
                                    )}
                                  >
                                    <LockOpenIcon fontSize="small" />
                                  </IconButton>
                                </span>
                              </Tooltip>
                            )}

                            <Tooltip
                              title={
                                isSelf
                                  ? t("errors:cannot_delete_self")
                                  : t("user:admin.actions.delete")
                              }
                            >
                              <span>
                                <IconButton
                                  size="small"
                                  color="error"
                                  onClick={() => openDelete(u)}
                                  disabled={isSelf || submitting}
                                  aria-label={t("user:admin.actions.delete")}
                                >
                                  <DeleteIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          </Stack>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Paper>
      </Stack>

      <UserEditorDialog mode={editor} onClose={() => setEditor(null)} />

      <Dialog
        open={confirm?.kind === "activate"}
        onClose={closeConfirm}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {confirm?.kind === "activate" &&
            (confirm.active
              ? t("user:admin.dialogs.activate.title", {
                  login: getUserName(confirm.target),
                })
              : t("user:admin.dialogs.deactivate.title", {
                  login: getUserName(confirm.target),
                }))}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirm?.kind === "activate" &&
              (confirm.active
                ? t("user:admin.dialogs.activate.message")
                : t("user:admin.dialogs.deactivate.message"))}
          </DialogContentText>
          {actionError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {actionError}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeConfirm} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            color={
              confirm?.kind === "activate" && !confirm.active
                ? "warning"
                : "primary"
            }
            onClick={() => void submitConfirm()}
            disabled={submitting}
          >
            {confirm?.kind === "activate" && confirm.active
              ? t("user:admin.dialogs.activate.submit")
              : t("user:admin.dialogs.deactivate.submit")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={confirm?.kind === "delete"}
        onClose={closeConfirm}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {confirm?.kind === "delete" &&
            t("user:admin.dialogs.delete.title", {
              login: getUserName(confirm.target),
            })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("user:admin.dialogs.delete.message")}
          </DialogContentText>
          {actionError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {actionError}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeConfirm} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={() => void submitConfirm()}
            disabled={submitting}
          >
            {t("user:admin.dialogs.delete.submit")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
