import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
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
import TuneIcon from "@mui/icons-material/Tune";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import {
  useCreateVehicleTemplate,
  useVehicleTemplates,
} from "../api/functions";

export default function VehicleTemplatesPage() {
  const { t } = useTranslation(["function", "errors", "common"]);
  const navigate = useNavigate();
  const templates = useVehicleTemplates();
  const create = useCreateVehicleTemplate();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const mutationError = (() => {
    const err = create.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const submitCreate = () => {
    create.mutate(
      { name: name.trim(), description: description.trim() },
      {
        onSuccess: (row) => {
          setDialogOpen(false);
          setName("");
          setDescription("");
          navigate(`/my/vehicle-templates/${row.id}/functions`);
        },
      },
    );
  };

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Stack spacing={2}>
        <Typography variant="h4">{t("function:templates.title")}</Typography>
        <Typography variant="body1" color="text.secondary">
          {t("function:templates.intro")}
        </Typography>

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
            }}
          >
            <Typography variant="h6" sx={{ flexGrow: 1 }}>
              {t("function:templates.listTitle")}
            </Typography>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => setDialogOpen(true)}
            >
              {t("function:templates.add")}
            </Button>
          </Box>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t("function:templates.columns.name")}</TableCell>
                  <TableCell>{t("function:templates.columns.description")}</TableCell>
                  <TableCell align="right">
                    {t("function:templates.columns.actions")}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {templates.isLoading ? (
                  <TableRow>
                    <TableCell colSpan={3} align="center" sx={{ py: 3 }}>
                      {t("common:loading")}
                    </TableCell>
                  </TableRow>
                ) : (templates.data ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3} align="center" sx={{ py: 3 }}>
                      {t("function:templates.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  (templates.data ?? []).map((row) => (
                    <TableRow key={row.id}>
                      <TableCell>{row.name}</TableCell>
                      <TableCell>{row.description || "—"}</TableCell>
                      <TableCell align="right">
                        <Tooltip title={t("function:templates.editFunctions")}>
                          <IconButton
                            size="small"
                            onClick={() =>
                              navigate(`/my/vehicle-templates/${row.id}/functions`)
                            }
                            aria-label={t("function:templates.editFunctions")}
                          >
                            <TuneIcon fontSize="small" />
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

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{t("function:templates.createTitle")}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            <TextField
              label={t("function:templates.fieldName")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              fullWidth
              required
            />
            <TextField
              label={t("function:templates.fieldDescription")}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              fullWidth
              multiline
              minRows={2}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>
            {t("function:editor.cancel")}
          </Button>
          <Button
            variant="contained"
            onClick={submitCreate}
            disabled={!name.trim() || create.isPending}
          >
            {t("function:templates.createSubmit")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
