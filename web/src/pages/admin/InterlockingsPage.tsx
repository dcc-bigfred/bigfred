import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
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
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import {
  useCreateInterlocking,
  useDeleteInterlocking,
  useInterlockings,
  useUpdateInterlocking,
  type Interlocking,
} from "../../api/interlockings";

export default function InterlockingsPage() {
  const { t } = useTranslation(["interlocking", "common", "errors"]);
  const list = useInterlockings();
  const create = useCreateInterlocking();
  const update = useUpdateInterlocking();
  const remove = useDeleteInterlocking();

  type DialogState =
    | { kind: "create" }
    | { kind: "edit"; target: Interlocking }
    | { kind: "delete"; target: Interlocking }
    | null;

  const [dialog, setDialog] = useState<DialogState>(null);
  const [nameInput, setNameInput] = useState("");
  const [locationInput, setLocationInput] = useState("");
  const [actionError, setActionError] = useState<string | null>(null);

  const closeDialog = () => {
    setDialog(null);
    setNameInput("");
    setLocationInput("");
    setActionError(null);
    create.reset();
    update.reset();
    remove.reset();
  };

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
    setNameInput("");
    setLocationInput("");
    setActionError(null);
  };

  const openEdit = (target: Interlocking) => {
    setDialog({ kind: "edit", target });
    setNameInput(target.name);
    setLocationInput(target.location);
    setActionError(null);
  };

  const openDelete = (target: Interlocking) => {
    setDialog({ kind: "delete", target });
    setActionError(null);
  };

  const submitDialog = async () => {
    if (!dialog) return;
    try {
      if (dialog.kind === "create") {
        await create.mutateAsync({
          name: nameInput.trim(),
          location: locationInput.trim(),
        });
      } else if (dialog.kind === "edit") {
        await update.mutateAsync({
          id: dialog.target.id,
          name: nameInput.trim(),
          location: locationInput.trim(),
        });
      } else if (dialog.kind === "delete") {
        await remove.mutateAsync(dialog.target.id);
      }
      closeDialog();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const submitting = create.isPending || update.isPending || remove.isPending;

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
              {t("interlocking:admin.title")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t("interlocking:admin.subtitle")}
            </Typography>
          </Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openCreate}
            disabled={submitting}
          >
            {t("interlocking:admin.actions.create")}
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
                {t("interlocking:admin.empty")}
              </Typography>
            </Box>
          ) : (
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>{t("interlocking:admin.columns.name")}</TableCell>
                    <TableCell>{t("interlocking:admin.columns.location")}</TableCell>
                    <TableCell align="right">
                      {t("interlocking:admin.columns.actions")}
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={row.id} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {row.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {row.location || "—"}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <Stack
                          direction="row"
                          spacing={0.5}
                          justifyContent="flex-end"
                        >
                          <Tooltip title={t("interlocking:admin.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => openEdit(row)}
                              disabled={submitting}
                              aria-label={t("interlocking:admin.actions.edit")}
                            >
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title={t("interlocking:admin.actions.delete")}>
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => openDelete(row)}
                              disabled={submitting}
                              aria-label={t("interlocking:admin.actions.delete")}
                            >
                              <DeleteIcon fontSize="small" />
                            </IconButton>
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

      <Dialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        onClose={closeDialog}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          {dialog?.kind === "edit"
            ? t("interlocking:admin.dialogs.edit.title")
            : t("interlocking:admin.dialogs.create.title")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label={t("interlocking:admin.dialogs.fields.name")}
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              autoFocus
              fullWidth
              required
            />
            <TextField
              label={t("interlocking:admin.dialogs.fields.location")}
              value={locationInput}
              onChange={(e) => setLocationInput(e.target.value)}
              helperText={t("interlocking:admin.dialogs.fields.locationHelp")}
              fullWidth
              multiline
              minRows={2}
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
            disabled={submitting || nameInput.trim() === ""}
          >
            {dialog?.kind === "edit"
              ? t("common:actions.save")
              : t("common:actions.create")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={dialog?.kind === "delete"}
        onClose={closeDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {dialog?.kind === "delete" &&
            t("interlocking:admin.dialogs.delete.title", {
              name: dialog.target.name,
            })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("interlocking:admin.dialogs.delete.message")}
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
    </Container>
  );
}
