import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  Paper,
  Stack,
  Switch,
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
import TuneIcon from "@mui/icons-material/Tune";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import {
  useCreateVehicleTemplate,
  useFunctionCatalogue,
  useUpdateVehicleTemplate,
  useVehicleTemplates,
  type DccFunction,
} from "../api/functions";
import FunctionSummaryChips from "../components/functions/FunctionSummaryChips";

type TemplateTableRow = {
  rowKey: string;
  rowType: "template";
  id: number;
  ownerId: number;
  name: string;
  ownerLogin: string;
  description: string;
  functions: DccFunction[];
};

type LocomotiveTableRow = {
  rowKey: string;
  rowType: "locomotive";
  vehicleId: number;
  ownerId: number;
  name: string;
  ownerLogin: string;
  functions: DccFunction[];
};

type TableRow = TemplateTableRow | LocomotiveTableRow;

type DialogMode = "create" | "edit" | null;

export default function VehicleTemplatesPage() {
  const { t } = useTranslation(["function", "errors", "common"]);
  const navigate = useNavigate();
  const me = useMe().data;
  const isAdmin = me?.effectiveRole === "admin";
  const templates = useVehicleTemplates();
  const [showLocomotives, setShowLocomotives] = useState(false);
  const catalogue = useFunctionCatalogue(showLocomotives);
  const create = useCreateVehicleTemplate();
  const update = useUpdateVehicleTemplate();
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const canEditTemplate = (ownerId: number) =>
    me != null && (me.id === ownerId || isAdmin);

  const tableRows = useMemo((): TableRow[] => {
    const templateRows: TemplateTableRow[] = (templates.data ?? []).map(
      (row) => ({
        rowKey: `template-${row.id}`,
        rowType: "template",
        id: row.id,
        ownerId: row.ownerId,
        name: row.name,
        ownerLogin: row.ownerLogin,
        description: row.description,
        functions: row.functions ?? [],
      }),
    );
    if (!showLocomotives) {
      return templateRows;
    }
    const locoRows: LocomotiveTableRow[] = (catalogue.data ?? [])
      .map((entry) => ({
        rowKey: `vehicle-${entry.vehicleId}`,
        rowType: "locomotive" as const,
        vehicleId: entry.vehicleId,
        ownerId: entry.ownerId,
        name: entry.vehicleName,
        ownerLogin: entry.ownerLogin,
        functions: entry.functions,
      }))
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));
    return [...templateRows, ...locoRows];
  }, [templates.data, catalogue.data, showLocomotives]);

  const isLoading =
    templates.isLoading || (showLocomotives && catalogue.isLoading);

  const activeMutation = dialogMode === "edit" ? update : create;

  const mutationError = (() => {
    const err = activeMutation.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const closeDialog = () => {
    setDialogMode(null);
    setEditingId(null);
    setName("");
    setDescription("");
  };

  const openCreate = () => {
    setName("");
    setDescription("");
    setEditingId(null);
    setDialogMode("create");
  };

  const openEdit = (row: TemplateTableRow) => {
    setName(row.name);
    setDescription(row.description);
    setEditingId(row.id);
    setDialogMode("edit");
  };

  const submitCreate = () => {
    create.mutate(
      { name: name.trim(), description: description.trim() },
      {
        onSuccess: (row) => {
          closeDialog();
          navigate(`/vehicle-templates/${row.id}/functions`);
        },
      },
    );
  };

  const submitEdit = () => {
    if (editingId == null) return;
    update.mutate(
      {
        id: editingId,
        name: name.trim(),
        description: description.trim(),
      },
      { onSuccess: () => closeDialog() },
    );
  };

  const colCount = showLocomotives ? 5 : 4;

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Stack spacing={2}>
        <Typography variant="h4">{t("function:templates.title")}</Typography>
        <Typography variant="body1" color="text.secondary">
          {t("function:templates.intro")}
        </Typography>

        {mutationError && <Alert severity="error">{mutationError}</Alert>}

        <Paper variant="outlined">
          <Box
            sx={{
              px: 2,
              py: 1.5,
              borderBottom: 1,
              borderColor: "divider",
              display: "flex",
              alignItems: "center",
              flexWrap: "wrap",
              gap: 1,
            }}
          >
            <Typography variant="h6" sx={{ flexGrow: 1 }}>
              {t("function:templates.listTitle")}
            </Typography>
            <FormControlLabel
              control={
                <Switch
                  checked={showLocomotives}
                  onChange={(e) => setShowLocomotives(e.target.checked)}
                />
              }
              label={t("function:templates.showLocomotives")}
            />
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={openCreate}
            >
              {t("function:templates.add")}
            </Button>
          </Box>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t("function:templates.columns.type")}</TableCell>
                  <TableCell>{t("function:templates.columns.name")}</TableCell>
                  <TableCell>{t("function:templates.columns.owner")}</TableCell>
                  {showLocomotives ? (
                    <TableCell>{t("function:templates.columns.functions")}</TableCell>
                  ) : (
                    <TableCell>{t("function:templates.columns.description")}</TableCell>
                  )}
                  <TableCell align="right">
                    {t("function:templates.columns.actions")}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={colCount} align="center" sx={{ py: 3 }}>
                      {t("common:loading")}
                    </TableCell>
                  </TableRow>
                ) : tableRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={colCount} align="center" sx={{ py: 3 }}>
                      {showLocomotives
                        ? t("function:templates.emptyWithLocomotives")
                        : t("function:templates.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  tableRows.map((row) => (
                    <TableRow key={row.rowKey}>
                      <TableCell>
                        <Chip
                          size="small"
                          label={
                            row.rowType === "template"
                              ? t("function:templates.rowType.template")
                              : t("function:templates.rowType.locomotive")
                          }
                          color={row.rowType === "template" ? "primary" : "default"}
                          variant="outlined"
                        />
                      </TableCell>
                      <TableCell>{row.name}</TableCell>
                      <TableCell>{row.ownerLogin}</TableCell>
                      <TableCell>
                        {row.rowType === "template" ? (
                          showLocomotives ? (
                            row.functions.length > 0 ? (
                              <FunctionSummaryChips functions={row.functions} />
                            ) : (
                              "—"
                            )
                          ) : (
                            row.description || "—"
                          )
                        ) : row.functions.length > 0 ? (
                          <FunctionSummaryChips functions={row.functions} />
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell align="right">
                        {row.rowType === "template" ? (
                          <>
                            {canEditTemplate(row.ownerId) && (
                              <Tooltip title={t("function:templates.editTemplate")}>
                                <IconButton
                                  size="small"
                                  onClick={() => openEdit(row)}
                                  aria-label={t("function:templates.editTemplate")}
                                >
                                  <EditIcon fontSize="small" />
                                </IconButton>
                              </Tooltip>
                            )}
                            <Tooltip title={t("function:templates.editFunctions")}>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  navigate(`/vehicle-templates/${row.id}/functions`)
                                }
                                aria-label={t("function:templates.editFunctions")}
                              >
                                <TuneIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          </>
                        ) : me?.id === row.ownerId ? (
                          <Tooltip title={t("function:templates.editFunctions")}>
                            <IconButton
                              size="small"
                              onClick={() =>
                                navigate(`/my/vehicles/${row.vehicleId}/functions`)
                              }
                              aria-label={t("function:templates.editFunctions")}
                            >
                              <TuneIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      </Stack>

      <Dialog
        open={dialogMode != null}
        onClose={closeDialog}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          {dialogMode === "edit"
            ? t("function:templates.editTitle")
            : t("function:templates.createTitle")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            <TextField
              label={t("function:templates.fieldName")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              fullWidth
              required
            />
            <TextField
              label={t("function:templates.fieldDescription")}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              fullWidth
              multiline
              minRows={2}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog}>{t("function:editor.cancel")}</Button>
          <Button
            variant="contained"
            onClick={dialogMode === "edit" ? submitEdit : submitCreate}
            disabled={!name.trim() || activeMutation.isPending}
          >
            {dialogMode === "edit"
              ? t("function:templates.editSubmit")
              : t("function:templates.createSubmit")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
