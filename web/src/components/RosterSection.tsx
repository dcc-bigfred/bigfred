import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
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
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import { useMe } from "../api/auth";
import {
  useAddTrainToRoster,
  useAddVehicleToRoster,
  useDeleteTrain,
  useDeleteVehicle,
  useLayoutTrains,
  useLayoutVehicles,
  useMyTrains,
  useMyVehicles,
  useRemoveTrainFromRoster,
  useRemoveVehicleFromRoster,
  type RosterVehicle,
  type RosterTrain,
  type Train,
  type Vehicle,
} from "../api/vehicles";
import TrainDialog from "./TrainDialog";
import VehicleDialog from "./VehicleDialog";

interface Props {
  layoutId: number;
}

// RosterSection renders the two dashboard tables introduced in this
// milestone — the layout vehicle roster and the train roster — plus
// the management blocks for the caller's own catalogue. Per the user
// brief the section ships **two top-level buttons** (add vehicle / add
// train) and an extra **"Remove from layout"** affordance on every
// roster row the caller owns.
export default function RosterSection({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const me = useMe().data;

  // Catalogue + roster queries. The two roster queries subscribe to
  // `layout.vehiclesChanged` / `layout.trainsChanged` internally —
  // every add/remove fans out and the tables update without polling.
  const vehicles = useMyVehicles();
  const trains = useMyTrains();
  const layoutVehicles = useLayoutVehicles(layoutId);
  const layoutTrains = useLayoutTrains(layoutId);

  const addVehicleToRoster = useAddVehicleToRoster();
  const removeVehicleFromRoster = useRemoveVehicleFromRoster();
  const addTrainToRoster = useAddTrainToRoster();
  const removeTrainFromRoster = useRemoveTrainFromRoster();
  const deleteVehicleMut = useDeleteVehicle();
  const deleteTrainMut = useDeleteTrain();

  // Dialog state. We allow at most one dialog open at a time; opening
  // one closes the others through the `set*Open(false)` calls below.
  const [vehicleDialogOpen, setVehicleDialogOpen] = useState(false);
  const [editingVehicle, setEditingVehicle] = useState<Vehicle | null>(null);

  const [trainDialogOpen, setTrainDialogOpen] = useState(false);
  const [editingTrain, setEditingTrain] = useState<Train | null>(null);

  // Build a set of vehicle ids already pinned to the layout so the
  // "Add to layout" button can be greyed out for already-attached
  // rows.
  const vehicleOnLayout = useMemo(() => {
    const s = new Set<number>();
    (layoutVehicles.data ?? []).forEach((v) => s.add(v.id));
    return s;
  }, [layoutVehicles.data]);

  const trainOnLayout = useMemo(() => {
    const s = new Set<number>();
    (layoutTrains.data ?? []).forEach((tt) => s.add(tt.id));
    return s;
  }, [layoutTrains.data]);

  const mutationError = (() => {
    const err =
      addVehicleToRoster.error ??
      removeVehicleFromRoster.error ??
      addTrainToRoster.error ??
      removeTrainFromRoster.error ??
      deleteVehicleMut.error ??
      deleteTrainMut.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const onDeleteVehicle = (v: Vehicle) => {
    if (!window.confirm(t("vehicle:list.deleteConfirm", { name: v.name }))) {
      return;
    }
    deleteVehicleMut.mutate(v.id);
  };

  const onDeleteTrain = (tr: Train) => {
    if (!window.confirm(t("vehicle:trainList.deleteConfirm", { name: tr.name }))) {
      return;
    }
    deleteTrainMut.mutate(tr.id);
  };

  const renderVehicleKind = (kind: Vehicle["kind"]) =>
    t(`vehicle:kind.${kind}` as const);

  const renderDCC = (vehicle: { dccAddress: number | null; isDummy?: boolean }) =>
    vehicle.dccAddress != null ? (
      String(vehicle.dccAddress)
    ) : (
      <Chip size="small" label={t("vehicle:dummyBadge")} />
    );

  const ownsRow = (ownerId: number) => me?.id === ownerId;

  return (
    <Stack spacing={3}>
      {mutationError && <Alert severity="error">{mutationError}</Alert>}

      {/* Roster — vehicles in the current layout */}
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
            {t("vehicle:roster.vehicles.title")}
          </Typography>
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:roster.vehicles.columns.name")}</TableCell>
                <TableCell>{t("vehicle:roster.vehicles.columns.kind")}</TableCell>
                <TableCell>{t("vehicle:roster.vehicles.columns.number")}</TableCell>
                <TableCell>{t("vehicle:roster.vehicles.columns.dccAddress")}</TableCell>
                <TableCell>{t("vehicle:roster.vehicles.columns.owner")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:roster.vehicles.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(layoutVehicles.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("vehicle:roster.vehicles.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (layoutVehicles.data ?? []).map((row: RosterVehicle) => (
                  <TableRow key={row.id}>
                    <TableCell>{row.name}</TableCell>
                    <TableCell>{renderVehicleKind(row.kind)}</TableCell>
                    <TableCell>{row.number || "—"}</TableCell>
                    <TableCell>{renderDCC(row)}</TableCell>
                    <TableCell>
                      {row.ownerLogin}
                      {ownsRow(row.ownerId) && (
                        <Typography component="span" variant="caption" color="text.secondary">
                          {" "}
                          {t("vehicle:roster.ownedByYou")}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      {ownsRow(row.ownerId) && (
                        <Tooltip title={t("vehicle:roster.removeButton")}>
                          <IconButton
                            size="small"
                            onClick={() =>
                              removeVehicleFromRoster.mutate({
                                layoutId,
                                vehicleId: row.id,
                              })
                            }
                            aria-label={t("vehicle:roster.removeButton")}
                          >
                            <RemoveCircleOutlineIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Roster — trains in the current layout */}
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
            {t("vehicle:roster.trains.title")}
          </Typography>
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:roster.trains.columns.name")}</TableCell>
                <TableCell>{t("vehicle:roster.trains.columns.members")}</TableCell>
                <TableCell>{t("vehicle:roster.trains.columns.owner")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:roster.trains.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(layoutTrains.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("vehicle:roster.trains.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (layoutTrains.data ?? []).map((row: RosterTrain) => (
                  <TableRow key={row.id}>
                    <TableCell>{row.name}</TableCell>
                    <TableCell>
                      {t("vehicle:trainList.membersCount", { count: row.members.length })}
                    </TableCell>
                    <TableCell>
                      {row.ownerLogin}
                      {ownsRow(row.ownerId) && (
                        <Typography component="span" variant="caption" color="text.secondary">
                          {" "}
                          {t("vehicle:roster.ownedByYou")}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      {ownsRow(row.ownerId) && (
                        <Tooltip title={t("vehicle:roster.removeButton")}>
                          <IconButton
                            size="small"
                            onClick={() =>
                              removeTrainFromRoster.mutate({
                                layoutId,
                                trainId: row.id,
                              })
                            }
                            aria-label={t("vehicle:roster.removeButton")}
                          >
                            <RemoveCircleOutlineIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Catalogue — own vehicles */}
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
            {t("vehicle:list.title")}
          </Typography>
          <Button
            startIcon={<AddIcon />}
            variant="contained"
            onClick={() => {
              setEditingVehicle(null);
              setVehicleDialogOpen(true);
            }}
          >
            {t("vehicle:list.addButton")}
          </Button>
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:list.columns.name")}</TableCell>
                <TableCell>{t("vehicle:list.columns.kind")}</TableCell>
                <TableCell>{t("vehicle:list.columns.number")}</TableCell>
                <TableCell>{t("vehicle:list.columns.dccAddress")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:list.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(vehicles.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("vehicle:list.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (vehicles.data ?? []).map((v) => {
                  const isOnLayout = vehicleOnLayout.has(v.id);
                  return (
                    <TableRow key={v.id}>
                      <TableCell>{v.name}</TableCell>
                      <TableCell>{renderVehicleKind(v.kind)}</TableCell>
                      <TableCell>{v.number || "—"}</TableCell>
                      <TableCell>{renderDCC(v)}</TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          <Tooltip
                            title={
                              isOnLayout
                                ? t("vehicle:list.actions.alreadyOnLayout")
                                : t("vehicle:list.actions.addToLayout")
                            }
                          >
                            <span>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  addVehicleToRoster.mutate({
                                    layoutId,
                                    vehicleId: v.id,
                                  })
                                }
                                disabled={isOnLayout || addVehicleToRoster.isPending}
                                aria-label={t("vehicle:list.actions.addToLayout")}
                              >
                                <PlaylistAddIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title={t("vehicle:list.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => {
                                setEditingVehicle(v);
                                setVehicleDialogOpen(true);
                              }}
                              aria-label={t("vehicle:list.actions.edit")}
                            >
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title={t("vehicle:list.actions.delete")}>
                            <IconButton
                              size="small"
                              onClick={() => onDeleteVehicle(v)}
                              aria-label={t("vehicle:list.actions.delete")}
                            >
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Catalogue — own trains */}
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
            {t("vehicle:trainList.title")}
          </Typography>
          <Button
            startIcon={<AddIcon />}
            variant="contained"
            onClick={() => {
              setEditingTrain(null);
              setTrainDialogOpen(true);
            }}
          >
            {t("vehicle:trainList.addButton")}
          </Button>
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:trainList.columns.name")}</TableCell>
                <TableCell>{t("vehicle:trainList.columns.members")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:trainList.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(trains.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("vehicle:trainList.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (trains.data ?? []).map((tr) => {
                  const isOnLayout = trainOnLayout.has(tr.id);
                  return (
                    <TableRow key={tr.id}>
                      <TableCell>{tr.name}</TableCell>
                      <TableCell>
                        {t("vehicle:trainList.membersCount", { count: tr.members.length })}
                      </TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          <Tooltip
                            title={
                              isOnLayout
                                ? t("vehicle:trainList.actions.alreadyOnLayout")
                                : t("vehicle:trainList.actions.addToLayout")
                            }
                          >
                            <span>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  addTrainToRoster.mutate({
                                    layoutId,
                                    trainId: tr.id,
                                  })
                                }
                                disabled={isOnLayout || addTrainToRoster.isPending}
                                aria-label={t("vehicle:trainList.actions.addToLayout")}
                              >
                                <PlaylistAddIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title={t("vehicle:trainList.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => {
                                setEditingTrain(tr);
                                setTrainDialogOpen(true);
                              }}
                              aria-label={t("vehicle:trainList.actions.edit")}
                            >
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title={t("vehicle:trainList.actions.delete")}>
                            <IconButton
                              size="small"
                              onClick={() => onDeleteTrain(tr)}
                              aria-label={t("vehicle:trainList.actions.delete")}
                            >
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      <VehicleDialog
        open={vehicleDialogOpen}
        vehicle={editingVehicle}
        onClose={() => setVehicleDialogOpen(false)}
      />
      <TrainDialog
        open={trainDialogOpen}
        train={editingTrain}
        onClose={() => setTrainDialogOpen(false)}
      />
    </Stack>
  );
}
