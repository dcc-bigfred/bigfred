import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Autocomplete,
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
import { useSearchParams } from "react-router-dom";

import { ApiError } from "../../api/client";
import {
  useAdminLayouts,
  useCreateLayout,
  useDeleteLayout,
  useSetLayoutLock,
  useUpdateLayout,
  type Layout,
} from "../../api/layouts";
import {
  useInterlockings,
  useLayoutInterlockings,
  type Interlocking,
} from "../../api/interlockings";

// LayoutsPage is the admin-only management screen for layouts
// (Polish: makiety) wired in §4.1 of the spec. It exposes the four
// CRUD operations + the lock toggle, while keeping the system
// layout's row read-only (its rename / lock / delete buttons are
// disabled with a tooltip pointing at the matching backend error
// code).
//
// All mutations route through TanStack hooks that invalidate both
// the admin list (`["layouts"]`) and the login dropdown
// (`["layouts","login"]`), so the login screen always sees a
// freshly-locked layout vanish on the next open.
export default function LayoutsPage() {
  const { t } = useTranslation(["layout", "common", "errors", "sudo"]);
  const [searchParams, setSearchParams] = useSearchParams();
  const list = useAdminLayouts();
  const interlockingsCatalog = useInterlockings();
  const create = useCreateLayout();
  const update = useUpdateLayout();
  const remove = useDeleteLayout();
  const setLock = useSetLayoutLock();

  type DialogState =
    | { kind: "create" }
    | { kind: "edit"; target: Layout }
    | { kind: "delete"; target: Layout }
    | { kind: "lock"; target: Layout; lock: boolean }
    | null;

  const [dialog, setDialog] = useState<DialogState>(null);
  const [editLayoutId, setEditLayoutId] = useState<number | null>(null);
  const layoutInterlockings = useLayoutInterlockings(editLayoutId);
  const [nameInput, setNameInput] = useState("");
  const [selectedInterlockings, setSelectedInterlockings] = useState<
    Interlocking[]
  >([]);
  const [adminPinInput, setAdminPinInput] = useState("");
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (dialog?.kind === "edit" && layoutInterlockings.data) {
      setSelectedInterlockings(layoutInterlockings.data);
    }
  }, [dialog, layoutInterlockings.data]);

  const closeDialog = () => {
    setDialog(null);
    setEditLayoutId(null);
    setNameInput("");
    setSelectedInterlockings([]);
    setAdminPinInput("");
    setActionError(null);
    create.reset();
    update.reset();
    remove.reset();
    setLock.reset();
  };

  // Translates an ApiError into a localised string. Falls back to
  // the generic "unknown" message so a code we don't translate
  // still surfaces a useful hint.
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
    setEditLayoutId(null);
    setNameInput("");
    setSelectedInterlockings([]);
    setAdminPinInput("");
    setActionError(null);
  };

  const openEdit = (target: Layout) => {
    setDialog({ kind: "edit", target });
    setEditLayoutId(target.id);
    setNameInput(target.name);
    setSelectedInterlockings([]);
    setAdminPinInput("");
    setActionError(null);
  };

  const editFromQuery = searchParams.get("edit");
  useEffect(() => {
    const id = Number(editFromQuery);
    if (!editFromQuery || Number.isNaN(id) || !list.data || dialog) return;
    const target = list.data.find((l) => l.id === id);
    if (!target) return;
    openEdit(target);
    setSearchParams({}, { replace: true });
  }, [editFromQuery, list.data, dialog, setSearchParams]);

  const openDelete = (target: Layout) => {
    setDialog({ kind: "delete", target });
    setActionError(null);
  };

  const openLock = (target: Layout, lock: boolean) => {
    setDialog({ kind: "lock", target, lock });
    setActionError(null);
  };

  const submitDialog = async () => {
    if (!dialog) return;
    try {
      const interlockingIds = selectedInterlockings.map((i) => i.id);
      const adminPin = adminPinInput.trim();
      if (dialog.kind === "create") {
        await create.mutateAsync({
          name: nameInput.trim(),
          interlockingIds,
          adminPin: adminPin === "" ? undefined : adminPin,
        });
      } else if (dialog.kind === "edit") {
        const name = dialog.target.isSystem
          ? dialog.target.name
          : nameInput.trim();
        await update.mutateAsync({
          id: dialog.target.id,
          name,
          interlockingIds,
          adminPin: adminPin === "" ? undefined : adminPin,
        });
      } else if (dialog.kind === "delete") {
        await remove.mutateAsync(dialog.target.id);
      } else if (dialog.kind === "lock") {
        await setLock.mutateAsync({ id: dialog.target.id, lock: dialog.lock });
      }
      closeDialog();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const interlockingOptions = useMemo(
    () => interlockingsCatalog.data ?? [],
    [interlockingsCatalog.data],
  );

  const submitting =
    create.isPending ||
    update.isPending ||
    remove.isPending ||
    setLock.isPending;

  const showSystemBadge = (l: Layout) =>
    l.isSystem ? t("layout:admin.type.system") : t("layout:admin.type.custom");

  const renderName = (l: Layout) =>
    l.isSystem ? t("layout:system_default_label") : l.name;

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
              {t("layout:admin.title")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t("layout:admin.subtitle")}
            </Typography>
          </Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openCreate}
            disabled={submitting}
          >
            {t("layout:admin.actions.create")}
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
                {t("layout:admin.empty")}
              </Typography>
            </Box>
          ) : (
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>{t("layout:admin.columns.name")}</TableCell>
                    <TableCell>{t("layout:admin.columns.type")}</TableCell>
                    <TableCell>{t("layout:admin.columns.status")}</TableCell>
                    <TableCell align="right">
                      {t("layout:admin.columns.actions")}
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((l) => (
                    <TableRow key={l.id} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {renderName(l)}
                        </Typography>
                        {l.isSystem && (
                          <Typography variant="caption" color="text.secondary">
                            {l.name}
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={l.isSystem ? "primary" : "default"}
                          label={showSystemBadge(l)}
                        />
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          color={l.locked ? "warning" : "success"}
                          label={
                            l.locked
                              ? t("layout:admin.status.locked")
                              : t("layout:admin.status.active")
                          }
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Stack
                          direction="row"
                          spacing={0.5}
                          justifyContent="flex-end"
                        >
                          <Tooltip title={t("layout:admin.actions.edit")}>
                            <span>
                              <IconButton
                                size="small"
                                onClick={() => openEdit(l)}
                                disabled={submitting}
                                aria-label={t("layout:admin.actions.edit")}
                              >
                                <EditIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>

                          {/* Lock/Unlock — disabled on the system row */}
                          {l.locked ? (
                            <Tooltip title={t("layout:admin.actions.unlock")}>
                              <span>
                                <IconButton
                                  size="small"
                                  onClick={() => openLock(l, false)}
                                  disabled={l.isSystem || submitting}
                                  aria-label={t("layout:admin.actions.unlock")}
                                >
                                  <LockOpenIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          ) : (
                            <Tooltip
                              title={
                                l.isSystem
                                  ? t("errors:default_layout_cannot_be_locked")
                                  : t("layout:admin.actions.lock")
                              }
                            >
                              <span>
                                <IconButton
                                  size="small"
                                  onClick={() => openLock(l, true)}
                                  disabled={l.isSystem || submitting}
                                  aria-label={t("layout:admin.actions.lock")}
                                >
                                  <LockIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          )}

                          {/* Delete — disabled on the system row */}
                          <Tooltip
                            title={
                              l.isSystem
                                ? t("errors:default_layout_undeletable")
                                : t("layout:admin.actions.delete")
                            }
                          >
                            <span>
                              <IconButton
                                size="small"
                                color="error"
                                onClick={() => openDelete(l)}
                                disabled={l.isSystem || submitting}
                                aria-label={t("layout:admin.actions.delete")}
                              >
                                <DeleteIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Paper>
      </Stack>

      {/* Create / Edit dialog. Edit replaces the old rename-only flow
          and adds a multi-select of interlockings whitelisted for the
          layout. The system row keeps its name read-only. */}
      <Dialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        onClose={closeDialog}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          {dialog?.kind === "edit"
            ? t("layout:admin.dialogs.edit.title")
            : t("layout:admin.dialogs.create.title")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label={
                dialog?.kind === "edit"
                  ? t("layout:admin.dialogs.edit.nameLabel")
                  : t("layout:admin.dialogs.create.nameLabel")
              }
              value={
                dialog?.kind === "edit" && dialog.target.isSystem
                  ? t("layout:system_default_label")
                  : nameInput
              }
              onChange={(e) => setNameInput(e.target.value)}
              helperText={
                dialog?.kind === "create"
                  ? t("layout:admin.dialogs.create.nameHelp")
                  : dialog?.kind === "edit" && dialog.target.isSystem
                    ? t("errors:default_layout_immutable")
                    : undefined
              }
              disabled={dialog?.kind === "edit" && dialog.target.isSystem}
              autoFocus={dialog?.kind !== "edit" || !dialog.target.isSystem}
              fullWidth
              required
            />

            <Autocomplete
              multiple
              options={interlockingOptions}
              getOptionLabel={(option) => option.name}
              value={selectedInterlockings}
              onChange={(_event, value) => setSelectedInterlockings(value)}
              isOptionEqualToValue={(a, b) => a.id === b.id}
              loading={interlockingsCatalog.isLoading}
              disabled={interlockingsCatalog.isError}
              renderTags={(value, getTagProps) =>
                value.map((option, index) => (
                  <Chip
                    {...getTagProps({ index })}
                    key={option.id}
                    label={option.name}
                    size="small"
                  />
                ))
              }
              renderInput={(params) => (
                <TextField
                  {...params}
                  label={t("layout:admin.dialogs.edit.interlockingsLabel")}
                  helperText={t("layout:admin.dialogs.edit.interlockingsHelp")}
                />
              )}
            />

            {dialog?.kind === "edit" && layoutInterlockings.isLoading && (
              <Box sx={{ display: "flex", justifyContent: "center", py: 1 }}>
                <CircularProgress size={24} />
              </Box>
            )}

            <TextField
              label={t("sudo:settings.pinLabel")}
              value={adminPinInput}
              onChange={(e) =>
                setAdminPinInput(e.target.value.replace(/[^0-9]/g, "").slice(0, 8))
              }
              helperText={t("sudo:settings.pinHelp")}
              type="password"
              inputMode="numeric"
              autoComplete="off"
              fullWidth
              slotProps={{
                input: {
                  inputProps: {
                    pattern: "[0-9]*",
                    maxLength: 8,
                  },
                },
              }}
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
              (dialog?.kind === "create" && nameInput.trim() === "") ||
              (dialog?.kind === "edit" &&
                !dialog.target.isSystem &&
                nameInput.trim() === "") ||
              (dialog?.kind === "edit" && layoutInterlockings.isLoading)
            }
          >
            {dialog?.kind === "edit"
              ? t("common:actions.save")
              : t("common:actions.create")}
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
            t("layout:admin.dialogs.delete.title", {
              name: renderName(dialog.target),
            })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("layout:admin.dialogs.delete.message")}
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
            {t("common:actions.delete")}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Lock confirmation. Same dialog reused for the unlock flow —
          unlock is harmless enough that it could skip the confirm,
          but consistency wins. */}
      <Dialog
        open={dialog?.kind === "lock"}
        onClose={closeDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {dialog?.kind === "lock" &&
            t("layout:admin.dialogs.lock.title", {
              name: renderName(dialog.target),
            })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("layout:admin.dialogs.lock.message")}
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
            color={dialog?.kind === "lock" && dialog.lock ? "warning" : "primary"}
            onClick={submitDialog}
            disabled={submitting}
          >
            {dialog?.kind === "lock" && dialog.lock
              ? t("layout:admin.actions.lock")
              : t("layout:admin.actions.unlock")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
