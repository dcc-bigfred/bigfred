import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  FormControlLabel,
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
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import HandshakeIcon from "@mui/icons-material/Handshake";
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import RefreshIcon from "@mui/icons-material/Refresh";
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import { lendableTargetKey, useGrantedLeases } from "../api/leases";
import {
  useAddTrainToRoster,
  useDeleteTrain,
  useRemoveTrainFromRoster,
  useTrainCatalogue,
  type CatalogueTrain,
} from "../api/vehicles";
import { getUserName } from "../utils/getUserName";
import {
  isTargetLeased,
  isTrainLendable,
  showLendButton,
  trainLendTooltip,
} from "../utils/lendAction";
import {
  canAddToLayout,
  canRemoveFromLayout,
  hasEffectiveAdmin,
} from "../utils/rosterPermissions";
import LeaseCreateDialog from "./leases/LeaseCreateDialog";
import TrainDialog from "./TrainDialog";

const ROWS_PER_PAGE = 10;

interface Props {
  layoutId: number;
}

export default function AvailableTrainsCatalogue({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common", "rentals"]);
  const me = useMe().data;
  const trains = useTrainCatalogue(layoutId);
  const addTrainToRoster = useAddTrainToRoster();
  const removeTrainFromRoster = useRemoveTrainFromRoster();
  const deleteTrainMut = useDeleteTrain();
  const grantedLeases = useGrantedLeases();

  const [mineOnly, setMineOnly] = useState(true);
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTrain, setEditingTrain] = useState<CatalogueTrain | null>(null);
  const [leaseDialogOpen, setLeaseDialogOpen] = useState(false);
  const [leaseInitialTarget, setLeaseInitialTarget] = useState<{
    kind: "train";
    targetId: string;
    targetName?: string;
  } | null>(null);

  useEffect(() => {
    setPage(0);
  }, [query, mineOnly]);

  const isAdmin = hasEffectiveAdmin(me);
  const ownsRow = (ownerId: number) => me?.id === ownerId;
  const canMutateTrain = (ownerId: number) => ownsRow(ownerId) || isAdmin;

  const leasedTargetKeys = useMemo(() => {
    const s = new Set<string>();
    (grantedLeases.data ?? []).forEach((lease) => s.add(lendableTargetKey(lease)));
    return s;
  }, [grantedLeases.data]);

  const scopedTrains = useMemo(() => {
    let list = trains.data ?? [];
    if (mineOnly && me?.id != null) {
      list = list.filter((tr) => tr.ownerId === me.id);
    }
    return list;
  }, [trains.data, mineOnly, me?.id]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      return scopedTrains;
    }
    return scopedTrains.filter((tr) => {
      const ownerLabel = getUserName({
        login: tr.ownerLogin,
        organization: tr.ownerOrganization,
      }).toLowerCase();
      const haystack = [tr.name, ownerLabel, String(tr.members.length)]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [scopedTrains, query]);

  const pagedRows = useMemo(() => {
    const start = page * ROWS_PER_PAGE;
    return filteredRows.slice(start, start + ROWS_PER_PAGE);
  }, [filteredRows, page]);

  const mutationError = (() => {
    const err =
      addTrainToRoster.error ??
      removeTrainFromRoster.error ??
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

  const onDeleteTrain = (tr: CatalogueTrain) => {
    if (!window.confirm(t("vehicle:trainList.deleteConfirm", { name: tr.name }))) {
      return;
    }
    deleteTrainMut.mutate(tr.id);
  };

  const onRefresh = () => {
    void trains.refetch();
    void grantedLeases.refetch();
  };

  const renderOnLayout = (onLayout: boolean) => (
    <Chip
      size="small"
      label={
        onLayout
          ? t("vehicle:trainCatalogue.onLayout.yes")
          : t("vehicle:trainCatalogue.onLayout.no")
      }
      color={onLayout ? "success" : "default"}
      variant={onLayout ? "filled" : "outlined"}
    />
  );

  const emptyMessage =
    (trains.data ?? []).length === 0
      ? t("vehicle:trainCatalogue.empty")
      : t("vehicle:trainCatalogue.noResults");

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
            flexDirection: "column",
            gap: 1.5,
          }}
        >
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Box sx={{ flexGrow: 1 }} />
            <FormControlLabel
              control={
                <Checkbox
                  size="small"
                  checked={mineOnly}
                  onChange={(e) => setMineOnly(e.target.checked)}
                />
              }
              label={t("vehicle:trainCatalogue.showOnlyMine")}
            />
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={onRefresh}
              disabled={trains.isFetching}
            >
              {t("vehicle:list.refreshButton")}
            </Button>
            <Button
              size="small"
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => {
                setEditingTrain(null);
                setDialogOpen(true);
              }}
            >
              {t("vehicle:trainList.addButton")}
            </Button>
          </Stack>
          <TextField
            fullWidth
            size="small"
            label={t("vehicle:trainCatalogue.searchLabel")}
            placeholder={t("vehicle:trainCatalogue.searchPlaceholder")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </Box>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:trainCatalogue.columns.name")}</TableCell>
                <TableCell>{t("vehicle:trainCatalogue.columns.members")}</TableCell>
                <TableCell>{t("vehicle:trainCatalogue.columns.onLayout")}</TableCell>
                <TableCell align="right">
                  {t("vehicle:trainCatalogue.columns.actions")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {trains.isLoading ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : pagedRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {emptyMessage}
                  </TableCell>
                </TableRow>
              ) : (
                pagedRows.map((tr) => {
                  const isOwner = ownsRow(tr.ownerId);
                  const leased = isTargetLeased(leasedTargetKeys, "train", tr.id);
                  const lendable = isTrainLendable(isAdmin, {
                    isOwner,
                    onLayout: tr.onLayout,
                    leased,
                  });
                  const lendTitle = trainLendTooltip(t, isAdmin, {
                    isOwner,
                    onLayout: tr.onLayout,
                    leased,
                  });
                  return (
                    <TableRow key={tr.id}>
                      <TableCell>
                        <Stack spacing={0.25}>
                          <Typography variant="body2">{tr.name}</Typography>
                          <Typography variant="caption" color="text.secondary" noWrap>
                            {getUserName({
                              login: tr.ownerLogin,
                              organization: tr.ownerOrganization,
                            })}
                          </Typography>
                        </Stack>
                      </TableCell>
                      <TableCell>
                        {t("vehicle:trainList.membersCount", { count: tr.members.length })}
                      </TableCell>
                      <TableCell>{renderOnLayout(tr.onLayout)}</TableCell>
                      <TableCell align="right">
                        <Stack
                          direction="row"
                          spacing={0.5}
                          justifyContent="flex-end"
                          flexWrap="wrap"
                          useFlexGap
                        >
                          {tr.onLayout ? (
                            canRemoveFromLayout(me, tr.ownerId) ? (
                              <Button
                                size="small"
                                variant="text"
                                startIcon={<RemoveCircleOutlineIcon />}
                                onClick={() =>
                                  removeTrainFromRoster.mutate({
                                    layoutId,
                                    trainId: tr.id,
                                  })
                                }
                                disabled={removeTrainFromRoster.isPending}
                              >
                                {t("vehicle:roster.removeButton")}
                              </Button>
                            ) : null
                          ) : canAddToLayout(me, tr.ownerId) ? (
                            <Button
                              size="small"
                              variant="text"
                              startIcon={<PlaylistAddIcon />}
                              onClick={() =>
                                addTrainToRoster.mutate({
                                  layoutId,
                                  trainId: tr.id,
                                })
                              }
                              disabled={addTrainToRoster.isPending}
                            >
                              {t("vehicle:trainList.actions.addToLayout")}
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
                                      kind: "train",
                                      targetId: tr.id,
                                      targetName: tr.name,
                                    });
                                    setLeaseDialogOpen(true);
                                  }}
                                >
                                  {t("rentals:granted.lend")}
                                </Button>
                              </span>
                            </Tooltip>
                          )}
                          {canMutateTrain(tr.ownerId) && (
                            <>
                              <Button
                                size="small"
                                variant="text"
                                startIcon={<EditIcon />}
                                onClick={() => {
                                  setEditingTrain(tr);
                                  setDialogOpen(true);
                                }}
                              >
                                {t("vehicle:trainList.actions.edit")}
                              </Button>
                              <Button
                                size="small"
                                variant="text"
                                color="error"
                                startIcon={<DeleteIcon />}
                                onClick={() => onDeleteTrain(tr)}
                              >
                                {t("vehicle:trainList.actions.delete")}
                              </Button>
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
            t("vehicle:trainCatalogue.pagination", { from, to, count })
          }
        />
      </Paper>

      <TrainDialog
        open={dialogOpen}
        train={editingTrain}
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
