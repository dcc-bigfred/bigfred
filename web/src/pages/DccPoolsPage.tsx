import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  Link,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import { useDccPools } from "../api/dccPools";
import { useUsers, type User } from "../api/users";
import {
  useClearVehicleDCC,
  useVehicleCatalogue,
  type CatalogueVehicle,
} from "../api/vehicles";
import UserEditorDialog, {
  type UserEditorDialogMode,
} from "../components/UserEditorDialog";
import VehicleDialog from "../components/VehicleDialog";
import { getUserName } from "../utils/getUserName";
import { hasEffectiveAdmin } from "../utils/rosterPermissions";

type PoolRange = {
  userId: number;
  login: string;
  organization: string;
  from: number;
  to: number;
};

type AddressRow = {
  kind: "address";
  key: string;
  userId: number;
  login: string;
  organization: string;
  address: number;
  vehicle: CatalogueVehicle | null;
};

type GapRow = {
  kind: "gap";
  key: string;
  userId: number;
  login: string;
  organization: string;
  skipped: number;
};

type SpacerRow = {
  kind: "spacer";
  key: string;
};

type TableRowModel = AddressRow | GapRow | SpacerRow;

function vehiclesByAddressInRange(
  catalogue: CatalogueVehicle[] | undefined,
  userId: number,
  from: number,
  to: number,
): Map<number, CatalogueVehicle> {
  const map = new Map<number, CatalogueVehicle>();
  if (!catalogue) return map;
  for (const v of catalogue) {
    if (v.ownerId !== userId || v.dccAddress == null) continue;
    if (v.dccAddress < from || v.dccAddress > to) continue;
    map.set(v.dccAddress, v);
  }
  return map;
}

/** Endpoints always; vehicle addresses when showVehicles. Gaps collapse to ellipsis. */
function expandPoolRows(
  range: PoolRange,
  byAddr: Map<number, CatalogueVehicle>,
  showVehicles: boolean,
): TableRowModel[] {
  const poolKey = `${range.userId}-${range.from}-${range.to}`;
  const anchors = new Set<number>([range.from, range.to]);
  if (showVehicles) {
    for (const addr of byAddr.keys()) {
      anchors.add(addr);
    }
  }
  const sorted = [...anchors].sort((a, b) => a - b);
  const out: TableRowModel[] = [];
  for (let i = 0; i < sorted.length; i++) {
    const addr = sorted[i]!;
    out.push({
      kind: "address",
      key: `${poolKey}-a-${addr}`,
      userId: range.userId,
      login: range.login,
      organization: range.organization,
      address: addr,
      vehicle: byAddr.get(addr) ?? null,
    });
    const next = sorted[i + 1];
    if (next == null) continue;
    const skipped = next - addr - 1;
    if (skipped > 0) {
      out.push({
        kind: "gap",
        key: `${poolKey}-g-${addr}-${next}`,
        userId: range.userId,
        login: range.login,
        organization: range.organization,
        skipped,
      });
    }
  }
  return out;
}

function userMatchesQuery(range: PoolRange, q: string): boolean {
  const display = getUserName({
    login: range.login,
    organization: range.organization,
  }).toLowerCase();
  return (
    display.includes(q) ||
    range.login.toLowerCase().includes(q) ||
    range.organization.toLowerCase().includes(q)
  );
}

function rangeHasMatchingVehicle(
  range: PoolRange,
  catalogue: CatalogueVehicle[] | undefined,
  q: string,
): boolean {
  if (!catalogue) return false;
  for (const v of catalogue) {
    if (v.ownerId !== range.userId || v.dccAddress == null) continue;
    if (v.dccAddress < range.from || v.dccAddress > range.to) continue;
    if (v.name.toLowerCase().includes(q)) return true;
  }
  return false;
}

export default function DccPoolsPage() {
  const { t } = useTranslation(["dccPool", "common", "errors"]);
  const me = useMe().data;
  const isAdmin = hasEffectiveAdmin(me);
  const [showVehicles, setShowVehicles] = useState(false);
  const [query, setQuery] = useState("");
  const [editor, setEditor] = useState<UserEditorDialogMode | null>(null);
  const [editingVehicle, setEditingVehicle] = useState<CatalogueVehicle | null>(
    null,
  );
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const searchQuery = query.trim().toLowerCase();
  const needCatalogue = showVehicles || searchQuery.length > 0;

  const pools = useDccPools();
  const users = useUsers({ enabled: isAdmin });
  const catalogue = useVehicleCatalogue(me?.layoutId ?? null, {
    enabled: needCatalogue,
  });
  const clearDcc = useClearVehicleDCC();

  const usersById = useMemo(() => {
    const map = new Map<number, User>();
    for (const u of users.data ?? []) {
      map.set(u.id, u);
    }
    return map;
  }, [users.data]);

  const ranges = useMemo((): PoolRange[] => {
    const out: PoolRange[] = [];
    for (const entry of pools.data ?? []) {
      for (const range of entry.dccPool) {
        out.push({
          userId: entry.userId,
          login: entry.login,
          organization: entry.organization,
          from: range.from,
          to: range.to,
        });
      }
    }
    out.sort((a, b) => a.from - b.from || a.to - b.to || a.userId - b.userId);
    if (!searchQuery) return out;
    return out.filter(
      (range) =>
        userMatchesQuery(range, searchQuery) ||
        rangeHasMatchingVehicle(range, catalogue.data, searchQuery),
    );
  }, [pools.data, searchQuery, catalogue.data]);

  const rows = useMemo((): TableRowModel[] => {
    const out: TableRowModel[] = [];
    ranges.forEach((range, index) => {
      if (index > 0) {
        out.push({
          kind: "spacer",
          key: `spacer-before-${range.userId}-${range.from}-${range.to}`,
        });
      }
      const byAddr = showVehicles
        ? vehiclesByAddressInRange(
            catalogue.data,
            range.userId,
            range.from,
            range.to,
          )
        : new Map<number, CatalogueVehicle>();
      out.push(...expandPoolRows(range, byAddr, showVehicles));
    });
    return out;
  }, [ranges, showVehicles, catalogue.data]);

  const openUserEditor = (userId: number) => {
    if (!isAdmin) return;
    const target = usersById.get(userId);
    if (!target) return;
    setEditor({ kind: "edit", target });
  };

  const canMutateVehicle = (ownerId: number) =>
    me?.id === ownerId || isAdmin;

  const openVehicleEditor = (vehicle: CatalogueVehicle) => {
    if (!canMutateVehicle(vehicle.ownerId)) return;
    setEditingVehicle(vehicle);
  };

  const setShowVehiclesChecked = (checked: boolean) => {
    setShowVehicles(checked);
    if (!checked) {
      setSelectedIds(new Set());
      setEditingVehicle(null);
      setActionError(null);
    }
  };

  const toggleSelected = (vehicleId: string, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(vehicleId);
      else next.delete(vehicleId);
      return next;
    });
  };

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const confirmRelease = async () => {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    try {
      await clearDcc.mutateAsync(ids);
      setSelectedIds(new Set());
      setConfirmOpen(false);
      setActionError(null);
    } catch (err) {
      setActionError(translateError(err));
      setConfirmOpen(false);
    }
  };

  const colCount = showVehicles ? 4 : 2;
  const selectedCount = selectedIds.size;

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Stack spacing={2}>
        <Box>
          <Typography variant="h5" component="h1">
            {t("dccPool:title")}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {t("dccPool:intro")}
          </Typography>
        </Box>

        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={2}
          alignItems={{ xs: "stretch", sm: "center" }}
        >
          <TextField
            size="small"
            label={t("dccPool:searchLabel")}
            placeholder={t("dccPool:searchPlaceholder")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            sx={{ flexGrow: 1, minWidth: { sm: 240 } }}
          />
          <FormControlLabel
            control={
              <Checkbox
                size="small"
                checked={showVehicles}
                onChange={(e) => setShowVehiclesChecked(e.target.checked)}
              />
            }
            label={t("dccPool:showVehicles")}
          />
          <Button
            variant="outlined"
            size="small"
            disabled={selectedCount === 0 || clearDcc.isPending}
            onClick={() => {
              setActionError(null);
              setConfirmOpen(true);
            }}
          >
            {t("dccPool:releaseSelected")}
          </Button>
        </Stack>

        {actionError && <Alert severity="error">{actionError}</Alert>}

        <Paper variant="outlined">
          {pools.isLoading ||
          (needCatalogue && catalogue.isLoading) ||
          (isAdmin && users.isLoading) ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress size={32} />
            </Box>
          ) : rows.length === 0 ? (
            <Typography color="text.secondary" sx={{ p: 2 }}>
              {searchQuery
                ? t("dccPool:noResults")
                : t("dccPool:empty")}
            </Typography>
          ) : (
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>{t("dccPool:columns.user")}</TableCell>
                    <TableCell>{t("dccPool:columns.address")}</TableCell>
                    {showVehicles && (
                      <TableCell>{t("dccPool:columns.vehicle")}</TableCell>
                    )}
                    {showVehicles && (
                      <TableCell padding="checkbox" align="right">
                        {t("dccPool:columns.select")}
                      </TableCell>
                    )}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((row) => {
                    if (row.kind === "spacer") {
                      return (
                        <TableRow key={row.key} aria-hidden>
                          <TableCell
                            colSpan={colCount}
                            sx={{
                              py: 1.5,
                              borderBottom: "none",
                              bgcolor: "action.hover",
                            }}
                          />
                        </TableRow>
                      );
                    }

                    const userName = getUserName({
                      login: row.login,
                      organization: row.organization,
                    });
                    const canEdit = isAdmin && usersById.has(row.userId);

                    if (row.kind === "gap") {
                      return (
                        <TableRow key={row.key}>
                          <TableCell>
                            {canEdit ? (
                              <Link
                                component="button"
                                type="button"
                                variant="body2"
                                color="text.secondary"
                                underline="hover"
                                onClick={() => openUserEditor(row.userId)}
                              >
                                {userName}
                              </Link>
                            ) : (
                              <Typography
                                variant="body2"
                                color="text.secondary"
                              >
                                {userName}
                              </Typography>
                            )}
                          </TableCell>
                          <TableCell colSpan={showVehicles ? 3 : 1}>
                            <Typography
                              variant="body2"
                              color="text.secondary"
                              fontStyle="italic"
                            >
                              {t("dccPool:gap", { count: row.skipped })}
                            </Typography>
                          </TableCell>
                        </TableRow>
                      );
                    }

                    return (
                      <TableRow key={row.key} hover>
                        <TableCell>
                          {canEdit ? (
                            <Link
                              component="button"
                              type="button"
                              variant="body2"
                              underline="hover"
                              onClick={() => openUserEditor(row.userId)}
                            >
                              {userName}
                            </Link>
                          ) : (
                            userName
                          )}
                        </TableCell>
                        <TableCell>
                          {canEdit ? (
                            <Link
                              component="button"
                              type="button"
                              variant="body2"
                              underline="hover"
                              onClick={() => openUserEditor(row.userId)}
                            >
                              {row.address}
                            </Link>
                          ) : (
                            <Typography variant="body2" component="span">
                              {row.address}
                            </Typography>
                          )}
                        </TableCell>
                        {showVehicles && (
                          <TableCell>
                            {row.vehicle ? (
                              canMutateVehicle(row.vehicle.ownerId) ? (
                                <Link
                                  component="button"
                                  type="button"
                                  variant="body2"
                                  underline="hover"
                                  onClick={() =>
                                    openVehicleEditor(row.vehicle!)
                                  }
                                >
                                  {row.vehicle.name}
                                </Link>
                              ) : (
                                <Typography variant="body2">
                                  {row.vehicle.name}
                                </Typography>
                              )
                            ) : (
                              <Typography
                                variant="body2"
                                color="text.secondary"
                              >
                                {t("dccPool:noVehicles")}
                              </Typography>
                            )}
                          </TableCell>
                        )}
                        {showVehicles && (
                          <TableCell padding="checkbox" align="right">
                            {row.vehicle ? (
                              <Checkbox
                                size="small"
                                checked={selectedIds.has(row.vehicle.id)}
                                onChange={(e) =>
                                  toggleSelected(
                                    row.vehicle!.id,
                                    e.target.checked,
                                  )
                                }
                                inputProps={{
                                  "aria-label": t("dccPool:columns.select"),
                                }}
                              />
                            ) : null}
                          </TableCell>
                        )}
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

      <VehicleDialog
        open={editingVehicle != null}
        vehicle={editingVehicle}
        onClose={() => setEditingVehicle(null)}
      />

      <Dialog
        open={confirmOpen}
        onClose={() => !clearDcc.isPending && setConfirmOpen(false)}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>{t("dccPool:releaseConfirm.title")}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("dccPool:releaseConfirm.body", { count: selectedCount })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setConfirmOpen(false)}
            disabled={clearDcc.isPending}
          >
            {t("dccPool:releaseConfirm.cancel")}
          </Button>
          <Button
            variant="contained"
            color="warning"
            onClick={() => void confirmRelease()}
            disabled={clearDcc.isPending || selectedCount === 0}
          >
            {t("dccPool:releaseConfirm.confirm")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
