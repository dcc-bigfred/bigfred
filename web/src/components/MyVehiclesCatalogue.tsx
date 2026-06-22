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
import TuneIcon from "@mui/icons-material/Tune";
import DeleteIcon from "@mui/icons-material/Delete";
import HandshakeIcon from "@mui/icons-material/Handshake";
import { useNavigate } from "react-router-dom";
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import { lendableTargetKey, useGrantedLeases } from "../api/leases";
import {
  useAddVehicleToRoster,
  useDeleteVehicle,
  useLayoutVehicles,
  useMyVehicles,
  useRemoveVehicleFromRoster,
  type Vehicle,
} from "../api/vehicles";
import LeaseCreateDialog from "./leases/LeaseCreateDialog";
import VehicleDialog from "./VehicleDialog";

interface Props {
  layoutId: number;
}

// MyVehiclesCatalogue is the caller's vehicle catalogue: CRUD plus
// "add to layout". Lives on /my/vehicles; the dashboard roster table
// stays in RosterSection.
export default function MyVehiclesCatalogue({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common", "rentals"]);
  const navigate = useNavigate();
  const vehicles = useMyVehicles();
  const layoutVehicles = useLayoutVehicles(layoutId);
  const addVehicleToRoster = useAddVehicleToRoster();
  const removeVehicleFromRoster = useRemoveVehicleFromRoster();
  const deleteVehicleMut = useDeleteVehicle();
  const grantedLeases = useGrantedLeases();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingVehicle, setEditingVehicle] = useState<Vehicle | null>(null);
  const [leaseDialogOpen, setLeaseDialogOpen] = useState(false);
  const [leaseInitialTarget, setLeaseInitialTarget] = useState<{
    kind: "vehicle";
    targetId: string;
  } | null>(null);

  const vehicleOnLayout = useMemo(() => {
    const s = new Set<string>();
    (layoutVehicles.data ?? []).forEach((v) => s.add(v.id));
    return s;
  }, [layoutVehicles.data]);

  const leasedTargetKeys = useMemo(() => {
    const s = new Set<string>();
    (grantedLeases.data ?? []).forEach((lease) => s.add(lendableTargetKey(lease)));
    return s;
  }, [grantedLeases.data]);

  const vehicleLendTooltip = (v: Vehicle, isOnLayout: boolean) => {
    const key = lendableTargetKey({ kind: "vehicle", targetId: v.id });
    if (leasedTargetKeys.has(key)) {
      return t("vehicle:list.actions.lendAlreadyLeased");
    }
    if (!isOnLayout) {
      return t("vehicle:list.actions.lendRequiresLayout");
    }
    if (v.dccAddress == null) {
      return t("vehicle:list.actions.lendRequiresDcc");
    }
    return t("rentals:granted.lend");
  };

  const canLendVehicle = (v: Vehicle, isOnLayout: boolean) => {
    const key = lendableTargetKey({ kind: "vehicle", targetId: v.id });
    return (
      isOnLayout &&
      v.dccAddress != null &&
      !leasedTargetKeys.has(key)
    );
  };

  const mutationError = (() => {
    const err =
      addVehicleToRoster.error ??
      removeVehicleFromRoster.error ??
      deleteVehicleMut.error;
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

  const renderVehicleKind = (kind: Vehicle["kind"]) =>
    t(`vehicle:kind.${kind}` as const);

  const renderDCC = (vehicle: { dccAddress: number | null }) =>
    vehicle.dccAddress != null ? (
      String(vehicle.dccAddress)
    ) : (
      <Chip size="small" label={t("vehicle:dummyBadge")} />
    );

  return (
    <>
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
              setDialogOpen(true);
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
              {vehicles.isLoading ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : (vehicles.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("vehicle:list.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (vehicles.data ?? []).map((v) => {
                  const isOnLayout = vehicleOnLayout.has(v.id);
                  const lendable = canLendVehicle(v, isOnLayout);
                  return (
                    <TableRow key={v.id}>
                      <TableCell>{v.name}</TableCell>
                      <TableCell>{renderVehicleKind(v.kind)}</TableCell>
                      <TableCell>{v.number || "—"}</TableCell>
                      <TableCell>{renderDCC(v)}</TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          {isOnLayout ? (
                            <Tooltip title={t("vehicle:roster.removeButton")}>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  removeVehicleFromRoster.mutate({
                                    layoutId,
                                    vehicleId: v.id,
                                  })
                                }
                                disabled={removeVehicleFromRoster.isPending}
                                aria-label={t("vehicle:roster.removeButton")}
                              >
                                <RemoveCircleOutlineIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          ) : (
                            <Tooltip title={t("vehicle:list.actions.addToLayout")}>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  addVehicleToRoster.mutate({
                                    layoutId,
                                    vehicleId: v.id,
                                  })
                                }
                                disabled={addVehicleToRoster.isPending}
                                aria-label={t("vehicle:list.actions.addToLayout")}
                              >
                                <PlaylistAddIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          )}
                          <Tooltip title={vehicleLendTooltip(v, isOnLayout)}>
                            <span>
                              <IconButton
                                size="small"
                                disabled={!lendable}
                                onClick={() => {
                                  setLeaseInitialTarget({
                                    kind: "vehicle",
                                    targetId: v.id,
                                  });
                                  setLeaseDialogOpen(true);
                                }}
                                aria-label={t("rentals:granted.lend")}
                              >
                                <HandshakeIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title={t("vehicle:list.actions.editFunctions")}>
                            <IconButton
                              size="small"
                              onClick={() => navigate(`/my/vehicles/${v.id}/functions`)}
                              aria-label={t("vehicle:list.actions.editFunctions")}
                            >
                              <TuneIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title={t("vehicle:list.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => {
                                setEditingVehicle(v);
                                setDialogOpen(true);
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

      <VehicleDialog
        open={dialogOpen}
        vehicle={editingVehicle}
        onClose={() => setDialogOpen(false)}
      />
      <LeaseCreateDialog
        open={leaseDialogOpen}
        onClose={() => setLeaseDialogOpen(false)}
        initialTarget={leaseInitialTarget}
      />
    </>
  );
}
