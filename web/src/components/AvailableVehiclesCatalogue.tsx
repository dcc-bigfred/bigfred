import { useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Tooltip,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import HandshakeIcon from "@mui/icons-material/Handshake";
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import RefreshIcon from "@mui/icons-material/Refresh";
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import TuneIcon from "@mui/icons-material/Tune";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import { lendableTargetKey, useGrantedLeases } from "../api/leases";
import {
  VEHICLE_EPOCHS,
  VEHICLE_KINDS,
  useAddVehicleToRoster,
  useDeleteVehicle,
  useRemoveVehicleFromRoster,
  useVehicleCatalogue,
  type CatalogueVehicle,
  type VehicleEpoch,
  type VehicleKind,
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
import VehiclesCatalogueTable, {
  type VehiclesCatalogueRow,
} from "./vehicles/VehiclesCatalogueTable";

const KIND_ALL = "";
const EPOCH_ALL = "__all__";
const EPOCH_NONE = "__none__";

interface Props {
  layoutId: number;
}

export default function AvailableVehiclesCatalogue({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common", "rentals"]);
  const navigate = useNavigate();
  const me = useMe().data;
  const vehicles = useVehicleCatalogue(layoutId);
  const addVehicleToRoster = useAddVehicleToRoster();
  const removeVehicleFromRoster = useRemoveVehicleFromRoster();
  const deleteVehicleMut = useDeleteVehicle();
  const grantedLeases = useGrantedLeases();

  const [mineOnly, setMineOnly] = useState(true);
  const [kindFilter, setKindFilter] = useState<string>(KIND_ALL);
  const [epochFilter, setEpochFilter] = useState<string>(EPOCH_ALL);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingVehicle, setEditingVehicle] = useState<CatalogueVehicle | null>(null);
  const [leaseDialogOpen, setLeaseDialogOpen] = useState(false);
  const [leaseInitialTarget, setLeaseInitialTarget] = useState<{
    kind: "vehicle";
    targetId: string;
    targetName?: string;
  } | null>(null);

  const isAdmin = hasEffectiveAdmin(me);
  const ownsRow = (ownerId: number) => me?.id === ownerId;
  const canMutateVehicle = (ownerId: number) => ownsRow(ownerId) || isAdmin;

  const leasedTargetKeys = useMemo(() => {
    const s = new Set<string>();
    (grantedLeases.data ?? []).forEach((lease) => s.add(lendableTargetKey(lease)));
    return s;
  }, [grantedLeases.data]);

  const filteredVehicles = useMemo(() => {
    let list = vehicles.data ?? [];
    if (mineOnly && me?.id != null) {
      list = list.filter((v) => v.ownerId === me.id);
    }
    if (kindFilter !== KIND_ALL) {
      list = list.filter((v) => v.kind === (kindFilter as VehicleKind));
    }
    if (epochFilter === EPOCH_NONE) {
      list = list.filter((v) => !v.epoch);
    } else if (epochFilter !== EPOCH_ALL) {
      list = list.filter((v) => v.epoch === (epochFilter as VehicleEpoch));
    }
    return list;
  }, [vehicles.data, mineOnly, me?.id, kindFilter, epochFilter]);

  const vehicleById = useMemo(() => {
    const m = new Map<string, CatalogueVehicle>();
    filteredVehicles.forEach((v) => m.set(v.id, v));
    return m;
  }, [filteredVehicles]);

  const rows: VehiclesCatalogueRow[] = useMemo(
    () =>
      filteredVehicles.map((v) => ({
        id: v.id,
        name: v.name,
        number: v.number,
        dccAddress: v.dccAddress,
        onLayout: v.onLayout,
        epoch: v.epoch || undefined,
        ownerLabel: getUserName({
          login: v.ownerLogin,
          organization: v.ownerOrganization,
        }),
        searchExtra: t(`vehicle:kind.${v.kind}` as const),
      })),
    [filteredVehicles, t],
  );

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

  const onRefresh = () => {
    void vehicles.refetch();
    void grantedLeases.refetch();
  };

  const headerExtra = (
    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
      <FormControlLabel
        control={
          <Checkbox
            size="small"
            checked={mineOnly}
            onChange={(e) => setMineOnly(e.target.checked)}
          />
        }
        label={t("vehicle:catalogue.showOnlyMine")}
      />
      <FormControl size="small" sx={{ minWidth: 140 }}>
        <InputLabel id="vehicle-kind-filter-label">
          {t("vehicle:catalogue.filterKind")}
        </InputLabel>
        <Select
          labelId="vehicle-kind-filter-label"
          label={t("vehicle:catalogue.filterKind")}
          value={kindFilter}
          onChange={(e) => setKindFilter(e.target.value)}
        >
          <MenuItem value={KIND_ALL}>{t("vehicle:catalogue.filterAll")}</MenuItem>
          {VEHICLE_KINDS.map((k) => (
            <MenuItem key={k} value={k}>
              {t(`vehicle:kind.${k}` as const)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl size="small" sx={{ minWidth: 140 }}>
        <InputLabel id="vehicle-epoch-filter-label">
          {t("vehicle:catalogue.filterEpoch")}
        </InputLabel>
        <Select
          labelId="vehicle-epoch-filter-label"
          label={t("vehicle:catalogue.filterEpoch")}
          value={epochFilter}
          onChange={(e) => setEpochFilter(e.target.value)}
        >
          <MenuItem value={EPOCH_ALL}>{t("vehicle:catalogue.filterAll")}</MenuItem>
          <MenuItem value={EPOCH_NONE}>{t("vehicle:catalogue.filterNoEpoch")}</MenuItem>
          {VEHICLE_EPOCHS.map((e) => (
            <MenuItem key={e} value={e}>
              {e}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <Button
        size="small"
        variant="outlined"
        startIcon={<RefreshIcon />}
        onClick={onRefresh}
        disabled={vehicles.isFetching}
      >
        {t("vehicle:list.refreshButton")}
      </Button>
      <Button
        size="small"
        variant="contained"
        startIcon={<AddIcon />}
        onClick={() => {
          setEditingVehicle(null);
          setDialogOpen(true);
        }}
      >
        {t("vehicle:list.addButton")}
      </Button>
    </Stack>
  );

  return (
    <>
      <VehiclesCatalogueTable
        rows={rows}
        loading={vehicles.isLoading}
        mutationError={mutationError}
        showSearch
        showOnLayoutChip
        headerExtra={headerExtra}
        emptyLabel={t("vehicle:catalogue.empty")}
        noResultsLabel={t("vehicle:catalogue.noResults")}
        sourceCount={(vehicles.data ?? []).length}
        renderActions={(row) => {
          const v = vehicleById.get(row.id);
          if (!v) return null;
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
            <>
              {v.onLayout ? (
                canRemoveFromLayout(me, v.ownerId) ? (
                  <Button
                    size="small"
                    variant="text"
                    startIcon={<RemoveCircleOutlineIcon />}
                    onClick={() =>
                      removeVehicleFromRoster.mutate({
                        layoutId,
                        vehicleId: v.id,
                      })
                    }
                    disabled={removeVehicleFromRoster.isPending}
                  >
                    {t("vehicle:roster.removeButton")}
                  </Button>
                ) : null
              ) : canAddToLayout(me, v.ownerId) ? (
                <Button
                  size="small"
                  variant="text"
                  startIcon={<PlaylistAddIcon />}
                  onClick={() =>
                    addVehicleToRoster.mutate({
                      layoutId,
                      vehicleId: v.id,
                    })
                  }
                  disabled={addVehicleToRoster.isPending}
                >
                  {t("vehicle:list.actions.addToLayout")}
                </Button>
              ) : null}
              {showLendButton(isAdmin, isOwner) && (
                <Tooltip title={lendTitle}>
                  <span>
                    <Button
                      size="small"
                      variant="text"
                      startIcon={<HandshakeIcon />}
                      disabled={!lendable}
                      onClick={() => {
                        setLeaseInitialTarget({
                          kind: "vehicle",
                          targetId: v.id,
                          targetName: v.name,
                        });
                        setLeaseDialogOpen(true);
                      }}
                    >
                      {t("rentals:granted.lend")}
                    </Button>
                  </span>
                </Tooltip>
              )}
              {canMutateVehicle(v.ownerId) && (
                <>
                  <Button
                    size="small"
                    variant="text"
                    startIcon={<TuneIcon />}
                    onClick={() => navigate(`/my/vehicles/${v.id}/functions`)}
                  >
                    {t("vehicle:list.actions.editFunctions")}
                  </Button>
                  <Button
                    size="small"
                    variant="text"
                    startIcon={<EditIcon />}
                    onClick={() => {
                      setEditingVehicle(v);
                      setDialogOpen(true);
                    }}
                  >
                    {t("vehicle:list.actions.edit")}
                  </Button>
                  <Button
                    size="small"
                    variant="text"
                    color="error"
                    startIcon={<DeleteIcon />}
                    onClick={() => onDeleteVehicle(v)}
                  >
                    {t("vehicle:list.actions.delete")}
                  </Button>
                </>
              )}
            </>
          );
        }}
      />

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
