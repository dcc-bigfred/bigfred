import {
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Chip,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";
import { useTranslation } from "react-i18next";

import { useFunctionCatalogue } from "../../api/functions";
import { FunctionIconVisual } from "./functionIconMap";

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function LocomotiveCatalogueDialog({ open, onClose }: Props) {
  const { t } = useTranslation(["function", "vehicle", "common"]);
  const catalogue = useFunctionCatalogue(open);

  const iconLabel = (slug: string) =>
    t(`function:icon.${slug}` as "function:icon.unspecified", {
      defaultValue: slug,
    });

  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth>
      <DialogTitle sx={{ pr: 6 }}>
        {t("function:catalogue.title")}
        <IconButton
          onClick={onClose}
          sx={{ position: "absolute", right: 8, top: 8 }}
          aria-label={t("function:editor.cancel")}
        >
          <CloseIcon />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {t("function:catalogue.intro")}
        </Typography>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("function:catalogue.columns.vehicle")}</TableCell>
                <TableCell>{t("function:catalogue.columns.owner")}</TableCell>
                <TableCell>{t("function:catalogue.columns.dcc")}</TableCell>
                <TableCell>{t("function:catalogue.columns.functions")}</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {catalogue.isLoading ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 3 }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : (catalogue.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 3 }}>
                    {t("function:catalogue.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                (catalogue.data ?? []).map((entry) => (
                  <TableRow key={entry.vehicleId}>
                    <TableCell>
                      <Typography variant="body2" fontWeight={600}>
                        {entry.vehicleName}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t(`vehicle:kind.${entry.kind}` as "vehicle:kind.loco")}
                      </Typography>
                    </TableCell>
                    <TableCell>{entry.ownerLogin}</TableCell>
                    <TableCell>
                      {entry.dccAddress != null ? (
                        String(entry.dccAddress)
                      ) : (
                        <Chip size="small" label={t("vehicle:dummyBadge")} />
                      )}
                    </TableCell>
                    <TableCell>
                      <Stack spacing={0.5}>
                        {[...entry.functions]
                          .sort((a, b) => a.position - b.position)
                          .map((f) => (
                            <Stack
                              key={f.num}
                              direction="row"
                              spacing={1}
                              alignItems="center"
                            >
                              <FunctionIconVisual icon={f.icon} />
                              <Typography variant="body2">
                                F{f.num}: {f.name} ({iconLabel(f.icon)})
                              </Typography>
                            </Stack>
                          ))}
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </DialogContent>
    </Dialog>
  );
}
