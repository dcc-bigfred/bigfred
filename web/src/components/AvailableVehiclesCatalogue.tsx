import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Chip,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import EditIcon from "@mui/icons-material/Edit";
import DeleteIcon from "@mui/icons-material/Delete";
import HandshakeIcon from "@mui/icons-material/Handshake";
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import { lendableTargetKey, useGrantedLeases } from "../api/leases";
import {
  useAddVehicleToRoster,
  useDeleteVehicle,
  useRemoveVehicleFromRoster,
  useVehicleCatalogue,
  type CatalogueVehicle,
} from "../api/vehicles";
import { getUserName } from "../utils/getUserName";
import {
  isTargetLeased,
  isVehicleLendable,
  showLendButton,
  vehicleLendTooltip,
} from "../utils/lendAction";
import {
  canAddToLayout,
  canRemoveFromLayout,
  hasEffectiveAdmin,
} from "../utils/rosterPermissions";
import LeaseCreateDialog from "./leases/LeaseCreateDialog";
import VehicleDialog from "./VehicleDialog";

const ROWS_PER_PAGE = 10;

interface Props {
  layoutId: number;
}

export default function AvailableVehiclesCatalogue({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common", "rentals"]);
  const me = useMe().data;
  const vehicles = useVehicleCatalogue(layoutId);
  const addVehicleToRoster = useAddVehicleToRoster();
  const removeVehicleFromRoster = useRemoveVehicleFromRoster();
  const deleteVehicleMut = useDeleteVehicle();
  const grantedLeases = useGrantedLeases();

  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingVehicle, setEditingVehicle] = useState<CatalogueVehicle | null>(null);
  const [leaseDialogOpen, setLeaseDialogOpen] = useState(false);
  const [leaseInitialTarget, setLeaseInitialTarget] = useState<{
    kind: "vehicle";
    targetId: string;
  } | null>(null);

  useEffect(() => {
    setPage(0);
  }, [query]);

  const isAdmin = hasEffectiveAdmin(me);
  const ownsRow = (ownerId: number) => me?.id === ownerId;
  const canMutateVehicle = (ownerId: number) => ownsRow(ownerId) || isAdmin;

  const leasedTargetKeys = useMemo(() => {
    const s = new Set<string>();
    (grantedLeases.data ?? []).forEach((lease) => s.add(lendableTargetKey(lease)));
    return s;
  }, [grantedLeases.data]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    const rows = vehicles.data ?? [];
    if (!q) {
      return rows;
    }
    return rows.filter((v) => {
      const ownerLabel = getUserName({
        login: v.ownerLogin,
        organization: v.ownerOrganization,
      }).toLowerCase();
      const kindLabel = t(`vehicle:kind.${v.kind}` as const).toLowerCase();
      const haystack = [
        v.name,
        v.number,
        v.dccAddress != null ? String(v.dccAddress) : "",
        kindLabel,
        ownerLabel,
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [vehicles.data, query, t]);

  const pagedRows = useMemo(() => {
    const start = page * ROWS_PER_PAGE;
    return filteredRows.slice(start, start + ROWS_PER_PAGE);
  }, [filteredRows, page]);

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

  const onDeleteVehicle = (v: CatalogueVehicle) => {
    if (!window.confirm(t("vehicle:list.deleteConfirm", { name: v.name }))) {
      return;
    }
    deleteVehicleMut.mutate(v.id);
  };

  const renderVehicleKind = (kind: CatalogueVehicle["kind"]) =>
    t(`vehicle:kind.${kind}` as const);

  const renderDCC = (vehicle: { dccAddress: number | null }) =>
    vehicle.dccAddress != null ? (
      String(vehicle.dccAddress)
    ) : (
      <Chip size="small" label={t("vehicle:dummyBadge")} />
    );

  const renderOnLayout = (onLayout: boolean) => (
    <Chip
      size="small"
      label={
        onLayout
          ? t("vehicle:catalogue.onLayout.yes")
          : t("vehicle:catalogue.onLayout.no")
      }
      color={onLayout ? "success" : "default"}
      variant={onLayout ? "filled" : "outlined"}
    />
  );

  const emptyMessage =
    (vehicles.data ?? []).length === 0
      ? t("vehicle:catalogue.empty")
      : t("vehicle:catalogue.noResults");

  return (
    <>
      {mutationError && <Alert severity="error">{mutationError}</Alert>}

      <Paper variant="outlined">
        <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}>
          <TextField
            fullWidth
            size="small"
            label={t("vehicle:catalogue.searchLabel")}
            placeholder={t("vehicle:catalogue.searchPlaceholder")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:catalogue.columns.name")}</TableCell>
                <TableCell>{t("vehicle:catalogue.columns.kind")}</TableCell>
                <TableCell>{t("vehicle:catalogue.columns.number")}</TableCell>
                <TableCell>{t("vehicle:catalogue.columns.dccAddress")}</TableCell>
                <TableCell>{t("vehicle:catalogue.columns.onLayout")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:catalogue.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {vehicles.isLoading ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : pagedRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {emptyMessage}
                  </TableCell>
                </TableRow>
              ) : (
                pagedRows.map((v) => {
                  const isOwner = ownsRow(v.ownerId);
                  const leased = isTargetLeased(leasedTargetKeys, "vehicle", v.id);
                  const lendable = isVehicleLendable(isAdmin, {
                    isOwner,
                    onLayout: v.onLayout,
                    dccAddress: v.dccAddress,
                    leased,
                  });
                  const lendTitle = vehicleLendTooltip(t, isAdmin, {
                    isOwner,
                    onLayout: v.onLayout,
                    dccAddress: v.dccAddress,
                    leased,
                  });
                  return (
                    <TableRow key={v.id}>
                      <TableCell>
                        <Stack spacing={0.25}>
                          <Typography variant="body2">{v.name}</Typography>
                          <Typography variant="caption" color="text.secondary" noWrap>
                            {getUserName({
                              login: v.ownerLogin,
                              organization: v.ownerOrganization,
                            })}
                          </Typography>
                        </Stack>
                      </TableCell>
                      <TableCell>{renderVehicleKind(v.kind)}</TableCell>
                      <TableCell>{v.number || "—"}</TableCell>
                      <TableCell>{renderDCC(v)}</TableCell>
                      <TableCell>{renderOnLayout(v.onLayout)}</TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          {v.onLayout ? (
                            canRemoveFromLayout(me, v.ownerId) ? (
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
                            ) : null
                          ) : canAddToLayout(me, v.ownerId) ? (
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
                          ) : null}
                          {showLendButton(isAdmin, isOwner) && (
                            <Tooltip title={lendTitle}>
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
                          )}
                          {canMutateVehicle(v.ownerId) && (
                            <>
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
                            </>
                          )}
                        </Stack>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={filteredRows.length}
          page={page}
          onPageChange={(_event, nextPage) => setPage(nextPage)}
          rowsPerPage={ROWS_PER_PAGE}
          rowsPerPageOptions={[ROWS_PER_PAGE]}
          labelDisplayedRows={({ from, to, count }) =>
            t("vehicle:catalogue.pagination", { from, to, count })
          }
        />
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
        allowUnresolvedTarget={isAdmin}
      />
    </>
  );
}
