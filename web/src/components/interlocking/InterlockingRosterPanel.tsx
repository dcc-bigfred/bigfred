import { useMemo, useState } from "react";
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Paper,
  Snackbar,
  Alert,
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
import SettingsInputAntennaIcon from "@mui/icons-material/SettingsInputAntenna";
import StopCircleOutlinedIcon from "@mui/icons-material/StopCircleOutlined";
import SwapHorizIcon from "@mui/icons-material/SwapHoriz";
import { useTranslation } from "react-i18next";

import {
  useLayoutTrains,
  useLayoutVehicles,
  type RosterTrain,
  type RosterVehicle,
} from "../../api/vehicles";
import type { RadioSendContext, RadioSendTarget } from "../../api/radio";
import { useTakeoverActions } from "../../api/takeover";
import { useEstopTargetActions } from "../../api/estop";
import { useDccBusOptional } from "../../context/DccBusContext";
import { getUserName } from "../../utils/getUserName";
import RadioPhrasePickerDialog from "./RadioPhrasePickerDialog";

interface InterlockingRosterPanelProps {
  layoutId: number;
}

type RosterRow =
  | {
      kind: "vehicle";
      key: string;
      ownerUserId: number;
      entityId: string;
      login: string;
      organization: string;
      name: string;
      addresses: number[];
    }
  | {
      kind: "train";
      key: string;
      ownerUserId: number;
      entityId: string;
      login: string;
      organization: string;
      name: string;
      addresses: number[];
    };

function vehicleRow(v: RosterVehicle): RosterRow | null {
  if (v.dccAddress == null) {
    return null;
  }
  return {
    kind: "vehicle",
    key: `v-${v.id}`,
    ownerUserId: v.ownerId,
    entityId: v.id,
      login: v.ownerLogin,
      organization: v.ownerOrganization,
      name: v.name,
    addresses: [v.dccAddress],
  };
}

function trainRow(
  t: RosterTrain,
  addrByVehicle: Map<string, number>,
): RosterRow {
  const addresses: number[] = [];
  for (const m of t.members) {
    const addr = addrByVehicle.get(m.vehicleId);
    if (addr != null) {
      addresses.push(addr);
    }
  }
  return {
    kind: "train",
    key: `t-${t.id}`,
    ownerUserId: t.ownerId,
    entityId: t.id,
      login: t.ownerLogin,
      organization: t.ownerOrganization,
      name: t.name,
    addresses,
  };
}

// InterlockingRosterPanel lists layout vehicles and trains with a
// client-side search filter and in-motion chips from dcc-bus state.
export default function InterlockingRosterPanel({
  layoutId,
}: InterlockingRosterPanelProps) {
  const { t } = useTranslation("interlocking");
  const { requestTakeover } = useTakeoverActions();
  const { estopTarget } = useEstopTargetActions();
  const dcc = useDccBusOptional();
  const vehicles = useLayoutVehicles(layoutId).data ?? [];
  const trains = useLayoutTrains(layoutId).data ?? [];
  const [query, setQuery] = useState("");
  const [stopConfirm, setStopConfirm] = useState<{
    target: "vehicle" | "train";
    targetId: string;
    label: string;
  } | null>(null);
  const [stopBusy, setStopBusy] = useState(false);
  const [stopError, setStopError] = useState<string | null>(null);
  const [takeoverError, setTakeoverError] = useState<string | null>(null);
  const [radioTarget, setRadioTarget] = useState<{
    to: RadioSendTarget;
    context: RadioSendContext;
    targetLabel: string;
    contextLabel: string;
  } | null>(null);

  const rows = useMemo(() => {
    const addrByVehicle = new Map<string, number>();
    for (const v of vehicles) {
      if (v.dccAddress != null) {
        addrByVehicle.set(v.id, v.dccAddress);
      }
    }
    const out: RosterRow[] = [];
    for (const v of vehicles) {
      const row = vehicleRow(v);
      if (row) {
        out.push(row);
      }
    }
    for (const tr of trains) {
      out.push(trainRow(tr, addrByVehicle));
    }
    const q = query.trim().toLowerCase();
    if (!q) {
      return out;
    }
    return out.filter((row) => {
      const label = `(${getUserName(row)}) ${row.name}`.toLowerCase();
      return label.includes(q);
    });
  }, [vehicles, trains, query]);

  const isInMotion = (addresses: number[]) => {
    if (!dcc || addresses.length === 0) {
      return false;
    }
    return addresses.some((addr) => (dcc.states.get(addr)?.speed ?? 0) !== 0);
  };

  return (
    <Paper
      variant="outlined"
      sx={{
        display: "flex",
        flexDirection: "column",
        minHeight: 320,
        maxHeight: "min(70vh, 640px)",
        width: "100%",
      }}
    >
      <Box sx={{ p: 2, borderBottom: 1, borderColor: "divider" }}>
        <Typography variant="subtitle1" gutterBottom>
          {t("view.roster.title")}
        </Typography>
        <TextField
          size="small"
          fullWidth
          placeholder={t("view.roster.search")}
          value={query}
          onChange={(ev) => setQuery(ev.target.value)}
          aria-label={t("view.roster.search")}
        />
      </Box>
      <TableContainer sx={{ flex: 1 }}>
        <Table size="small" stickyHeader>
          <TableHead>
            <TableRow>
              <TableCell>{t("view.roster.columns.consist")}</TableCell>
              <TableCell align="right" width={120}>
                {t("view.roster.columns.actions")}
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={2}>
                  <Typography variant="body2" color="text.secondary">
                    {t("view.roster.empty")}
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={row.key} hover>
                  <TableCell>
                    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
                      <Typography variant="body2">
                        ({getUserName(row)}) {row.name}
                      </Typography>
                      {isInMotion(row.addresses) && (
                        <Chip
                          size="small"
                          color="warning"
                          label={t("view.roster.inMotion")}
                        />
                      )}
                    </Stack>
                  </TableCell>
                  <TableCell align="right">
                    <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                      <Tooltip title={t("view.roster.actions.radio")}>
                        <IconButton
                          size="small"
                          aria-label={t("view.roster.actions.radio")}
                          onClick={() =>
                            setRadioTarget({
                              to: { userId: row.ownerUserId },
                              context:
                                row.kind === "vehicle"
                                  ? { vehicleId: row.entityId }
                                  : { trainId: row.entityId },
                              targetLabel: getUserName(row),
                              contextLabel: row.name,
                            })
                          }
                        >
                          <SettingsInputAntennaIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title={t("view.roster.actions.stopHint")}>
                        <IconButton
                          size="small"
                          color="error"
                          aria-label={t("view.roster.actions.stop")}
                          onClick={() =>
                            setStopConfirm({
                              target: row.kind === "vehicle" ? "vehicle" : "train",
                              targetId: row.entityId,
                              label: `(${getUserName(row)}) ${row.name}`,
                            })
                          }
                        >
                          <StopCircleOutlinedIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title={t("view.roster.actions.takeover")}>
                        <IconButton
                          size="small"
                          aria-label={t("view.roster.actions.takeover")}
                          onClick={() => {
                            void requestTakeover(
                              row.kind === "vehicle" ? "vehicle" : "train",
                              row.entityId,
                            )
                              .then((ack) => {
                                if (!ack.ok) {
                                  setTakeoverError(ack.error ?? "error");
                                }
                              })
                              .catch(() => setTakeoverError("error"));
                          }}
                        >
                          <SwapHorizIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </Stack>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
      {radioTarget && (
        <RadioPhrasePickerDialog
          open
          onClose={() => setRadioTarget(null)}
          to={radioTarget.to}
          context={radioTarget.context}
          side="signalman"
          targetLabel={radioTarget.targetLabel}
          contextLabel={radioTarget.contextLabel}
        />
      )}
      <Dialog open={stopConfirm != null} onClose={() => setStopConfirm(null)}>
        <DialogTitle>{t("view.roster.stopConfirm.title")}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("view.roster.stopConfirm.message", { target: stopConfirm?.label ?? "" })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setStopConfirm(null)} disabled={stopBusy}>
            {t("view.roster.stopConfirm.cancel")}
          </Button>
          <Button
            color="error"
            variant="contained"
            disabled={stopBusy}
            onClick={() => {
              if (!stopConfirm) {
                return;
              }
              const req = stopConfirm;
              setStopBusy(true);
              void estopTarget(req.target, req.targetId)
                .then((ack) => {
                  if (!ack.ok) {
                    setStopError(ack.error ?? "error");
                  }
                })
                .catch(() => setStopError("error"))
                .finally(() => {
                  setStopBusy(false);
                  setStopConfirm(null);
                });
            }}
          >
            {t("view.roster.stopConfirm.confirm")}
          </Button>
        </DialogActions>
      </Dialog>
      <Snackbar
        open={stopError != null}
        autoHideDuration={5000}
        onClose={() => setStopError(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert severity="error" onClose={() => setStopError(null)} variant="filled">
          {t(`view.roster.stopError.${stopError ?? "error"}`, {
            defaultValue: t("view.roster.stopError.error"),
          })}
        </Alert>
      </Snackbar>
      <Snackbar
        open={takeoverError != null}
        autoHideDuration={5000}
        onClose={() => setTakeoverError(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert
          severity="error"
          onClose={() => setTakeoverError(null)}
          variant="filled"
        >
          {t(`view.roster.takeoverError.${takeoverError ?? "error"}`, {
            defaultValue: t("view.roster.takeoverError.error"),
          })}
        </Alert>
      </Snackbar>
    </Paper>
  );
}
