import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Radio,
  RadioGroup,
  Select,
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
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import {
  useFunctionIcons,
  type DccFunction,
  type FunctionKind,
  type FunctionUpsertBody,
} from "../../api/functions";
import { FunctionIconVisual } from "./functionIconMap";
import LocomotiveCatalogueDialog from "./LocomotiveCatalogueDialog";

export type FunctionEditorMode = "vehicle" | "template";

interface MutationHooks {
  upsert: {
    mutate: (args: { num: number; body: FunctionUpsertBody }) => void;
    isPending: boolean;
    error: Error | null;
  };
  remove: {
    mutate: (num: number) => void;
    isPending: boolean;
    error: Error | null;
  };
  reorder: {
    mutate: (positions: { num: number; position: number }[]) => void;
    isPending: boolean;
    error: Error | null;
  };
}

interface Props {
  mode: FunctionEditorMode;
  title: string;
  subtitle?: string;
  onBack: () => void;
  functions: DccFunction[] | undefined;
  isLoading: boolean;
  mutations: MutationHooks;
  showLocomotivesButton?: boolean;
  inheritedBanner?: boolean;
}

type EditState =
  | { kind: "add"; num: number }
  | { kind: "edit"; row: DccFunction }
  | null;

const FN_MAX = 31;

export default function FunctionListEditor({
  mode,
  title,
  subtitle,
  onBack,
  functions,
  isLoading,
  mutations,
  showLocomotivesButton = false,
  inheritedBanner = false,
}: Props) {
  const { t } = useTranslation(["function", "errors", "common", "vehicle"]);
  const icons = useFunctionIcons();
  const [edit, setEdit] = useState<EditState>(null);
  const [locomotivesOpen, setLocomotivesOpen] = useState(false);
  const [name, setName] = useState("");
  const [icon, setIcon] = useState("unspecified");
  const [kind, setKind] = useState<FunctionKind>("latched");

  const sorted = useMemo(
    () => [...(functions ?? [])].sort((a, b) => a.position - b.position),
    [functions],
  );

  const usedNums = useMemo(
    () => new Set(sorted.map((f) => f.num)),
    [sorted],
  );

  const freeNums = useMemo(() => {
    const out: number[] = [];
    for (let n = 0; n <= FN_MAX; n++) {
      if (!usedNums.has(n)) out.push(n);
    }
    return out;
  }, [usedNums]);

  const mutationError = (() => {
    const err =
      mutations.upsert.error ??
      mutations.remove.error ??
      mutations.reorder.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const openAdd = () => {
    if (freeNums.length === 0) return;
    setEdit({ kind: "add", num: freeNums[0] });
    setName("");
    setIcon("unspecified");
    setKind("latched");
  };

  const openEditRow = (row: DccFunction) => {
    setEdit({ kind: "edit", row });
    setName(row.name);
    setIcon(row.icon);
    setKind(row.kind);
  };

  const closeEdit = () => setEdit(null);

  const saveEdit = () => {
    if (!edit) return;
    const num = edit.kind === "add" ? edit.num : edit.row.num;
    const position =
      edit.kind === "edit"
        ? edit.row.position
        : sorted.length;
    mutations.upsert.mutate({
      num,
      body: { name: name.trim(), icon, kind, position },
    });
    closeEdit();
  };

  const moveRow = (index: number, direction: -1 | 1) => {
    const next = index + direction;
    if (next < 0 || next >= sorted.length) return;
    const reordered = [...sorted];
    const tmp = reordered[index];
    reordered[index] = reordered[next];
    reordered[next] = tmp;
    mutations.reorder.mutate(
      reordered.map((f, i) => ({ num: f.num, position: i })),
    );
  };

  const iconLabel = (slug: string) =>
    t(`function:icon.${slug}` as "function:icon.unspecified", {
      defaultValue: slug,
    });

  return (
    <>
      <Stack spacing={2}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Button startIcon={<ArrowBackIcon />} onClick={onBack} size="small">
            {t("function:editor.back")}
          </Button>
          <Box sx={{ flexGrow: 1 }}>
            <Typography variant="h5">{title}</Typography>
            {subtitle && (
              <Typography variant="body2" color="text.secondary">
                {subtitle}
              </Typography>
            )}
          </Box>
          {showLocomotivesButton && (
            <Button variant="outlined" onClick={() => setLocomotivesOpen(true)}>
              {t("function:editor.showLocomotives")}
            </Button>
          )}
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openAdd}
            disabled={freeNums.length === 0}
          >
            {t("function:editor.add")}
          </Button>
        </Stack>

        {inheritedBanner && (
          <Alert severity="info">{t("function:editor.inheritedFromTemplate")}</Alert>
        )}

        {mutationError && <Alert severity="error">{mutationError}</Alert>}

        <Paper variant="outlined">
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell width={56}>{t("function:editor.columns.order")}</TableCell>
                  <TableCell width={72}>{t("function:editor.columns.num")}</TableCell>
                  <TableCell>{t("function:editor.columns.title")}</TableCell>
                  <TableCell>{t("function:editor.columns.icon")}</TableCell>
                  <TableCell>{t("function:editor.columns.kind")}</TableCell>
                  {mode === "vehicle" && (
                    <TableCell>{t("function:editor.columns.source")}</TableCell>
                  )}
                  <TableCell align="right">{t("function:editor.columns.actions")}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={mode === "vehicle" ? 7 : 6} align="center" sx={{ py: 3 }}>
                      {t("common:loading")}
                    </TableCell>
                  </TableRow>
                ) : sorted.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={mode === "vehicle" ? 7 : 6} align="center" sx={{ py: 3 }}>
                      {t("function:editor.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  sorted.map((row, index) => (
                    <TableRow key={row.num}>
                      <TableCell>
                        <Stack direction="row" spacing={0}>
                          <IconButton
                            size="small"
                            disabled={index === 0 || mutations.reorder.isPending}
                            onClick={() => moveRow(index, -1)}
                            aria-label={t("function:editor.moveUp")}
                          >
                            <ArrowUpwardIcon fontSize="small" />
                          </IconButton>
                          <IconButton
                            size="small"
                            disabled={
                              index === sorted.length - 1 || mutations.reorder.isPending
                            }
                            onClick={() => moveRow(index, 1)}
                            aria-label={t("function:editor.moveDown")}
                          >
                            <ArrowDownwardIcon fontSize="small" />
                          </IconButton>
                        </Stack>
                      </TableCell>
                      <TableCell>F{row.num}</TableCell>
                      <TableCell>{row.name}</TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={1} alignItems="center">
                          <FunctionIconVisual icon={row.icon} />
                          <Typography variant="body2">{iconLabel(row.icon)}</Typography>
                        </Stack>
                      </TableCell>
                      <TableCell>
                        {row.kind === "latched"
                          ? t("function:kind.latched")
                          : t("function:kind.momentary")}
                      </TableCell>
                      {mode === "vehicle" && (
                        <TableCell>
                          {row.source === "template" ? (
                            <Chip size="small" label={t("function:editor.sourceTemplate")} />
                          ) : (
                            <Chip size="small" variant="outlined" label={t("function:editor.sourceVehicle")} />
                          )}
                        </TableCell>
                      )}
                      <TableCell align="right">
                        <Tooltip title={t("function:editor.editRow")}>
                          <IconButton size="small" onClick={() => openEditRow(row)}>
                            <EditIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={t("function:editor.deleteRow")}>
                          <IconButton
                            size="small"
                            onClick={() => {
                              if (
                                window.confirm(
                                  t("function:editor.deleteConfirm", { name: row.name }),
                                )
                              ) {
                                mutations.remove.mutate(row.num);
                              }
                            }}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      </Stack>

      <Dialog open={edit != null} onClose={closeEdit} maxWidth="sm" fullWidth>
        <DialogTitle>
          {edit?.kind === "add"
            ? t("function:editor.dialogAdd")
            : t("function:editor.dialogEdit")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            {edit?.kind === "add" && (
              <FormControl fullWidth>
                <InputLabel>{t("function:editor.fieldNum")}</InputLabel>
                <Select
                  label={t("function:editor.fieldNum")}
                  value={edit.num}
                  onChange={(e) =>
                    setEdit({ kind: "add", num: Number(e.target.value) })
                  }
                >
                  {freeNums.map((n) => (
                    <MenuItem key={n} value={n}>
                      F{n}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            )}
            {edit?.kind === "edit" && (
              <TextField
                label={t("function:editor.fieldNum")}
                value={`F${edit.row.num}`}
                disabled
                fullWidth
              />
            )}
            <FormControl fullWidth>
              <InputLabel>{t("function:editor.fieldIcon")}</InputLabel>
              <Select
                label={t("function:editor.fieldIcon")}
                value={icon}
                onChange={(e) => {
                  const nextIcon = e.target.value;
                  setIcon(nextIcon);
                  if (!name.trim()) {
                    setName(iconLabel(nextIcon));
                  }
                }}
                renderValue={(v) => (
                  <Stack direction="row" spacing={1} alignItems="center">
                    <FunctionIconVisual icon={v} />
                    <span>{iconLabel(v)}</span>
                  </Stack>
                )}
              >
                {(icons.data ?? [{ icon: "unspecified" }]).map((row) => (
                  <MenuItem key={row.icon} value={row.icon}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <FunctionIconVisual icon={row.icon} />
                      <span>{iconLabel(row.icon)}</span>
                    </Stack>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              label={t("function:editor.fieldTitle")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              fullWidth
              required
            />
            <FormControl>
              <Typography variant="subtitle2" gutterBottom>
                {t("function:editor.fieldKind")}
              </Typography>
              <RadioGroup
                value={kind}
                onChange={(e) => setKind(e.target.value as FunctionKind)}
              >
                <FormControlLabel
                  value="latched"
                  control={<Radio />}
                  label={t("function:kind.latched")}
                />
                <FormControlLabel
                  value="momentary"
                  control={<Radio />}
                  label={t("function:kind.momentary")}
                />
              </RadioGroup>
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeEdit}>{t("function:editor.cancel")}</Button>
          <Button
            variant="contained"
            onClick={saveEdit}
            disabled={!name.trim() || mutations.upsert.isPending}
          >
            {t("function:editor.save")}
          </Button>
        </DialogActions>
      </Dialog>

      <LocomotiveCatalogueDialog
        open={locomotivesOpen}
        onClose={() => setLocomotivesOpen(false)}
      />
    </>
  );
}
