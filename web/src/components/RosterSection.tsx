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
import RemoveCircleOutlineIcon from "@mui/icons-material/RemoveCircleOutline";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { ApiError } from "../api/client";
import { useMe } from "../api/auth";
import {
  useLayoutTrains,
  useLayoutVehicles,
  useRemoveTrainFromRoster,
  useRemoveVehicleFromRoster,
  type RosterVehicle,
  type RosterTrain,
} from "../api/vehicles";
import { getUserName } from "../utils/getUserName";
import { canRemoveFromLayout } from "../utils/rosterPermissions";

interface Props {
  layoutId: number;
}

// RosterSection renders the layout vehicle and train rosters on the
// dashboard. Catalogue CRUD lives on /my/vehicles and /my/trains.
export default function RosterSection({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const navigate = useNavigate();
  const me = useMe().data;

  const layoutVehicles = useLayoutVehicles(layoutId);
  const layoutTrains = useLayoutTrains(layoutId);

  const removeVehicleFromRoster = useRemoveVehicleFromRoster();
  const removeTrainFromRoster = useRemoveTrainFromRoster();

  const mutationError = (() => {
    const err = removeVehicleFromRoster.error ?? removeTrainFromRoster.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const renderVehicleKind = (kind: RosterVehicle["kind"]) =>
    t(`vehicle:kind.${kind}` as const);

  const renderDCC = (vehicle: { dccAddress: number | null; isDummy?: boolean }) =>
    vehicle.dccAddress != null ? (
      String(vehicle.dccAddress)
    ) : (
      <Chip size="small" label={t("vehicle:dummyBadge")} />
    );

  const ownsRow = (ownerId: number) => me?.id === ownerId;
  const canRemoveFromRoster = (ownerId: number) => canRemoveFromLayout(me, ownerId);

  return (
    <Stack spacing={3}>
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
            {t("vehicle:roster.trains.title")}
          </Typography>
          <Button
            variant="outlined"
            size="small"
            onClick={() => navigate("/my/trains")}
          >
            {t("vehicle:roster.trains.manageButton")}
          </Button>
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
                      {getUserName({
                        login: row.ownerLogin,
                        organization: row.ownerOrganization,
                      })}
                      {ownsRow(row.ownerId) && (
                        <Typography component="span" variant="caption" color="text.secondary">
                          {" "}
                          {t("vehicle:roster.ownedByYou")}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      {canRemoveFromRoster(row.ownerId) && (
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
          <Button
            variant="outlined"
            size="small"
            onClick={() => navigate("/my/vehicles")}
          >
            {t("vehicle:roster.vehicles.manageButton")}
          </Button>
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
                      {getUserName({
                        login: row.ownerLogin,
                        organization: row.ownerOrganization,
                      })}
                      {ownsRow(row.ownerId) && (
                        <Typography component="span" variant="caption" color="text.secondary">
                          {" "}
                          {t("vehicle:roster.ownedByYou")}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      {canRemoveFromRoster(row.ownerId) && (
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
    </Stack>
  );
}
