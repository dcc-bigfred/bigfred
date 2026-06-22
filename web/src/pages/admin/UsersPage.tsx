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
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
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
  USER_MANAGEABLE_ROLES,
  useCreateUser,
  useDeleteUser,
  useSetUserActive,
  useUpdateUser,
  useUsers,
  type User,
} from "../../api/users";
import UserDccPoolFields, {
  dccPoolFromApi,
  emptyDccPoolRange,
  formatDccPoolSummary,
  isDccPoolInputValid,
  parseDccPoolRanges,
  type DccPoolRangeInput,
} from "../../components/UserDccPoolFields";
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
  const create = useCreateUser();
  const update = useUpdateUser();
  const remove = useDeleteUser();
  const setActive = useSetUserActive();

  type DialogState =
    | { kind: "create" }
    | { kind: "edit"; target: User }
    | { kind: "delete"; target: User }
    | { kind: "activate"; target: User; active: boolean }
    | null;

  const [dialog, setDialog] = useState<DialogState>(null);
  const [loginInput, setLoginInput] = useState("");
  const [organizationInput, setOrganizationInput] = useState("");
  const [pinInput, setPinInput] = useState("");
  const [roleInput, setRoleInput] = useState<Role>("driver");
  const [dccPoolInput, setDccPoolInput] = useState<DccPoolRangeInput[]>([
    emptyDccPoolRange(),
  ]);
  const [actionError, setActionError] = useState<string | null>(null);

  const closeDialog = () => {
    setDialog(null);
    setLoginInput("");
    setOrganizationInput("");
    setPinInput("");
    setRoleInput("driver");
    setDccPoolInput([emptyDccPoolRange()]);
    setActionError(null);
    create.reset();
    update.reset();
    remove.reset();
    setActive.reset();
  };

  // Translates an ApiError into a localised string. Falls back to the
  // generic "unknown" message so an unmapped code still surfaces.
  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const openCreate = () => {
    setDialog({ kind: "create" });
    setLoginInput("");
    setOrganizationInput("");
    setPinInput("");
    setRoleInput("driver");
    setDccPoolInput([emptyDccPoolRange()]);
    setActionError(null);
  };

  const openEdit = (target: User) => {
    setDialog({ kind: "edit", target });
    setLoginInput(target.login);
    setOrganizationInput(target.organization ?? "");
    setPinInput("");
    setRoleInput(target.role);
    setDccPoolInput(dccPoolFromApi(target.dccPool ?? []));
    setActionError(null);
  };

  const openDelete = (target: User) => {
    setDialog({ kind: "delete", target });
    setActionError(null);
  };

  const openSetActive = (target: User, active: boolean) => {
    setDialog({ kind: "activate", target, active });
    setActionError(null);
  };

  const submitDialog = async () => {
    if (!dialog) return;
    try {
      if (dialog.kind === "create") {
        const dccPool = parseDccPoolRanges(dccPoolInput);
        if (!dccPool) return;
        await create.mutateAsync({
          login: loginInput.trim(),
          organization: organizationInput.trim(),
          pin: pinInput,
          role: roleInput,
          dccPool,
        });
      } else if (dialog.kind === "edit") {
        const dccPool = parseDccPoolRanges(dccPoolInput);
        if (!dccPool) return;
        await update.mutateAsync({
          id: dialog.target.id,
          login: loginInput.trim(),
          organization: organizationInput.trim(),
          role: roleInput,
          pin: pinInput || undefined,
          dccPool,
        });
      } else if (dialog.kind === "delete") {
        await remove.mutateAsync(dialog.target.id);
      } else if (dialog.kind === "activate") {
        await setActive.mutateAsync({
          id: dialog.target.id,
          active: dialog.active,
        });
      }
      closeDialog();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const submitting =
    create.isPending ||
    update.isPending ||
    remove.isPending ||
    setActive.isPending;

  const renderRole = (role: Role) =>
    t(`role:${role}` as const, { defaultValue: role });

  // Validation helpers used by the dialog submit gate so the button
  // disables before the request even leaves.
  const trimmedLogin = loginInput.trim();
  const loginValid = /^[A-Za-z0-9._-]{1,32}$/.test(trimmedLogin);
  const pinValid = /^[0-9]{4,12}$/.test(pinInput);
  const dccPoolValid = isDccPoolInputValid(dccPoolInput);
  const createValid = loginValid && pinValid && dccPoolValid;
  const editValid =
    loginValid && (pinInput === "" || pinValid) && dccPoolValid;

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
                                    aria-label={t("user:admin.actions.deactivate")}
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
                                    aria-label={t("user:admin.actions.activate")}
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

      {/* Create / Edit dialog. Edit hides the PIN helper so the admin
          knows leaving the field blank keeps the existing PIN. */}
      <Dialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        onClose={closeDialog}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          {dialog?.kind === "edit"
            ? t("user:admin.dialogs.edit.title", { login: getUserName(dialog.target) })
            : t("user:admin.dialogs.create.title")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label={t("user:admin.dialogs.fields.login")}
              value={loginInput}
              onChange={(e) => setLoginInput(e.target.value)}
              helperText={t("user:admin.dialogs.fields.loginHelp")}
              autoFocus
              fullWidth
              required
            />
            <TextField
              label={t("user:admin.dialogs.fields.organization")}
              value={organizationInput}
              onChange={(e) => setOrganizationInput(e.target.value)}
              helperText={t("user:admin.dialogs.fields.organizationHelp")}
              fullWidth
              inputProps={{ maxLength: 128 }}
            />
            <TextField
              select
              label={t("user:admin.dialogs.fields.role")}
              value={roleInput}
              onChange={(e) => setRoleInput(e.target.value as Role)}
              fullWidth
            >
              {USER_MANAGEABLE_ROLES.map((role) => (
                <MenuItem key={role} value={role}>
                  {renderRole(role)}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              label={t("user:admin.dialogs.fields.pin")}
              type="password"
              value={pinInput}
              onChange={(e) => setPinInput(e.target.value)}
              helperText={
                dialog?.kind === "edit"
                  ? t("user:admin.dialogs.fields.pinEditHelp")
                  : t("user:admin.dialogs.fields.pinCreateHelp")
              }
              placeholder={
                dialog?.kind === "edit"
                  ? t("user:admin.dialogs.fields.pinPlaceholder")
                  : undefined
              }
              required={dialog?.kind === "create"}
              fullWidth
              inputProps={{ inputMode: "numeric", pattern: "[0-9]*" }}
            />
            <UserDccPoolFields
              value={dccPoolInput}
              onChange={setDccPoolInput}
              disabled={submitting}
            />
            {actionError && <Alert severity="error">{actionError}</Alert>}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            onClick={submitDialog}
            disabled={
              submitting ||
              (dialog?.kind === "create" && !createValid) ||
              (dialog?.kind === "edit" && !editValid)
            }
          >
            {dialog?.kind === "edit"
              ? t("user:admin.dialogs.edit.submit")
              : t("user:admin.dialogs.create.submit")}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Activate / Deactivate confirmation. */}
      <Dialog
        open={dialog?.kind === "activate"}
        onClose={closeDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {dialog?.kind === "activate" &&
            (dialog.active
              ? t("user:admin.dialogs.activate.title", {
                  login: getUserName(dialog.target),
                })
              : t("user:admin.dialogs.deactivate.title", {
                  login: getUserName(dialog.target),
                }))}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {dialog?.kind === "activate" &&
              (dialog.active
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
          <Button onClick={closeDialog} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            color={
              dialog?.kind === "activate" && !dialog.active ? "warning" : "primary"
            }
            onClick={submitDialog}
            disabled={submitting}
          >
            {dialog?.kind === "activate" && dialog.active
              ? t("user:admin.dialogs.activate.submit")
              : t("user:admin.dialogs.deactivate.submit")}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete confirmation. */}
      <Dialog
        open={dialog?.kind === "delete"}
        onClose={closeDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {dialog?.kind === "delete" &&
            t("user:admin.dialogs.delete.title", {
              login: getUserName(dialog.target),
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
          <Button onClick={closeDialog} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={submitDialog}
            disabled={submitting}
          >
            {t("user:admin.dialogs.delete.submit")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
