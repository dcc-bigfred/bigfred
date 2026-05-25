import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
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
import {
  useAddTrainToRoster,
  useDeleteTrain,
  useLayoutTrains,
  useMyTrains,
  useRemoveTrainFromRoster,
  type Train,
} from "../api/vehicles";
import TrainDialog from "./TrainDialog";

interface Props {
  layoutId: number;
}

// MyTrainsCatalogue is the caller's train catalogue: CRUD plus
// "add to layout". Lives on /my/trains.
export default function MyTrainsCatalogue({ layoutId }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const trains = useMyTrains();
  const layoutTrains = useLayoutTrains(layoutId);
  const addTrainToRoster = useAddTrainToRoster();
  const removeTrainFromRoster = useRemoveTrainFromRoster();
  const deleteTrainMut = useDeleteTrain();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTrain, setEditingTrain] = useState<Train | null>(null);

  const trainOnLayout = useMemo(() => {
    const s = new Set<number>();
    (layoutTrains.data ?? []).forEach((tt) => s.add(tt.id));
    return s;
  }, [layoutTrains.data]);

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

  const onDeleteTrain = (tr: Train) => {
    if (!window.confirm(t("vehicle:trainList.deleteConfirm", { name: tr.name }))) {
      return;
    }
    deleteTrainMut.mutate(tr.id);
  };

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
            {t("vehicle:trainList.title")}
          </Typography>
          <Button
            startIcon={<AddIcon />}
            variant="contained"
            onClick={() => {
              setEditingTrain(null);
              setDialogOpen(true);
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
              {trains.isLoading ? (
                <TableRow>
                  <TableCell colSpan={3} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : (trains.data ?? []).length === 0 ? (
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
                          {isOnLayout ? (
                            <Tooltip title={t("vehicle:roster.removeButton")}>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  removeTrainFromRoster.mutate({
                                    layoutId,
                                    trainId: tr.id,
                                  })
                                }
                                disabled={removeTrainFromRoster.isPending}
                                aria-label={t("vehicle:roster.removeButton")}
                              >
                                <RemoveCircleOutlineIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          ) : (
                            <Tooltip title={t("vehicle:trainList.actions.addToLayout")}>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  addTrainToRoster.mutate({
                                    layoutId,
                                    trainId: tr.id,
                                  })
                                }
                                disabled={addTrainToRoster.isPending}
                                aria-label={t("vehicle:trainList.actions.addToLayout")}
                              >
                                <PlaylistAddIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          )}
                          <Tooltip title={t("vehicle:trainList.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => {
                                setEditingTrain(tr);
                                setDialogOpen(true);
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

      <TrainDialog
        open={dialogOpen}
        train={editingTrain}
        onClose={() => setDialogOpen(false)}
      />
    </>
  );
}
