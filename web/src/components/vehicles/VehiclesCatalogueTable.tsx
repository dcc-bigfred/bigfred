import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Alert,
  Box,
  Chip,
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
import { useTranslation } from "react-i18next";

export const VEHICLES_CATALOGUE_ROWS_PER_PAGE = 10;

export type VehiclesCatalogueRow = {
  id: string;
  name: string;
  number: string;
  dccAddress: number | null;
  onLayout: boolean;
  /** Shown under the name (Available catalogue). */
  ownerLabel?: string;
  /** Epoch label (e.g. "III"); empty/omitted → no chip. */
  epoch?: string;
  /** Extra text included in search haystack (e.g. kind label). */
  searchExtra?: string;
};

export type VehiclesCatalogueTableProps = {
  rows: VehiclesCatalogueRow[];
  loading: boolean;
  mutationError?: string | null;
  showSearch?: boolean;
  /** Extra controls in the header row (e.g. Add / Refresh / filters). */
  headerExtra?: ReactNode;
  /** Optional title next to headerExtra; when omitted, search/headerExtra fill the bar. */
  title?: string;
  emptyLabel: string;
  noResultsLabel?: string;
  showOnLayoutChip?: boolean;
  renderActions: (row: VehiclesCatalogueRow) => ReactNode;
};

function VehicleCatalogueRowView({
  row,
  showOnLayoutChip,
  actions,
}: {
  row: VehiclesCatalogueRow;
  showOnLayoutChip: boolean;
  actions: ReactNode;
}) {
  const { t } = useTranslation(["vehicle"]);

  const dccNode =
    row.dccAddress != null ? (
      <Typography variant="body2" color="text.secondary" component="span">
        {t("vehicle:catalogue.dccLabel", { addr: row.dccAddress })}
      </Typography>
    ) : (
      <Chip size="small" label={t("vehicle:dummyBadge")} />
    );

  return (
    <TableRow>
      <TableCell sx={{ verticalAlign: "top", width: "40%" }}>
        <Stack spacing={0.25}>
          <Tooltip title={t("vehicle:idTooltip", { id: row.id })}>
            <Typography variant="body2" component="span" sx={{ display: "inline-block" }}>
              {row.name}
            </Typography>
          </Tooltip>
          {row.ownerLabel ? (
            <Typography variant="caption" color="text.secondary" noWrap>
              {row.ownerLabel}
            </Typography>
          ) : null}
          <Box sx={{ pt: 0.25 }}>{dccNode}</Box>
        </Stack>
      </TableCell>
      <TableCell sx={{ verticalAlign: "top" }}>
        <Stack spacing={1} alignItems="flex-end">
          <Stack
            direction="row"
            spacing={1}
            flexWrap="wrap"
            useFlexGap
            justifyContent="flex-end"
            alignItems="center"
          >
            <Typography variant="body2" color="text.secondary">
              {row.number || "—"}
            </Typography>
            {showOnLayoutChip ? (
              <Chip
                size="small"
                label={
                  row.onLayout
                    ? t("vehicle:catalogue.onLayout.yes")
                    : t("vehicle:catalogue.onLayout.no")
                }
                color={row.onLayout ? "success" : "default"}
                variant={row.onLayout ? "filled" : "outlined"}
              />
            ) : null}
            {row.epoch ? (
              <Chip
                size="small"
                color="info"
                label={t("vehicle:catalogue.epochChip", { epoch: row.epoch })}
              />
            ) : null}
          </Stack>
          <Stack direction="row" spacing={0.5} justifyContent="flex-end" flexWrap="wrap" useFlexGap>
            {actions}
          </Stack>
        </Stack>
      </TableCell>
    </TableRow>
  );
}

export default function VehiclesCatalogueTable({
  rows,
  loading,
  mutationError,
  showSearch = true,
  headerExtra,
  title,
  emptyLabel,
  noResultsLabel,
  showOnLayoutChip = false,
  renderActions,
}: VehiclesCatalogueTableProps) {
  const { t } = useTranslation(["vehicle", "common"]);
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);

  useEffect(() => {
    setPage(0);
  }, [query, rows]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      return rows;
    }
    return rows.filter((row) => {
      const haystack = [
        row.name,
        row.number,
        row.dccAddress != null ? String(row.dccAddress) : "",
        row.ownerLabel ?? "",
        row.epoch ?? "",
        row.searchExtra ?? "",
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [rows, query]);

  const pagedRows = useMemo(() => {
    const start = page * VEHICLES_CATALOGUE_ROWS_PER_PAGE;
    return filteredRows.slice(start, start + VEHICLES_CATALOGUE_ROWS_PER_PAGE);
  }, [filteredRows, page]);

  const emptyMessage =
    rows.length === 0 ? emptyLabel : (noResultsLabel ?? emptyLabel);

  const showHeaderBar = Boolean(title || headerExtra || showSearch);

  return (
    <>
      {mutationError ? <Alert severity="error">{mutationError}</Alert> : null}

      <Paper variant="outlined">
        {showHeaderBar ? (
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
            {(title || headerExtra) && (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  flexWrap: "wrap",
                }}
              >
                {title ? (
                  <Typography variant="h6" sx={{ flexGrow: 1 }}>
                    {title}
                  </Typography>
                ) : (
                  <Box sx={{ flexGrow: 1 }} />
                )}
                {headerExtra}
              </Box>
            )}
            {showSearch ? (
              <TextField
                fullWidth
                size="small"
                label={t("vehicle:catalogue.searchLabel")}
                placeholder={t("vehicle:catalogue.searchPlaceholder")}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            ) : null}
          </Box>
        ) : null}

        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>{t("vehicle:catalogue.table.vehicle")}</TableCell>
                <TableCell align="right">{t("vehicle:catalogue.table.details")}</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={2} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : pagedRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={2} align="center" sx={{ py: 3, color: "text.secondary" }}>
                    {emptyMessage}
                  </TableCell>
                </TableRow>
              ) : (
                pagedRows.map((row) => (
                  <VehicleCatalogueRowView
                    key={row.id}
                    row={row}
                    showOnLayoutChip={showOnLayoutChip}
                    actions={renderActions(row)}
                  />
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <TablePagination
          component="div"
          count={filteredRows.length}
          page={page}
          onPageChange={(_event, nextPage) => setPage(nextPage)}
          rowsPerPage={VEHICLES_CATALOGUE_ROWS_PER_PAGE}
          rowsPerPageOptions={[VEHICLES_CATALOGUE_ROWS_PER_PAGE]}
          labelDisplayedRows={({ from, to, count }) =>
            t("vehicle:catalogue.pagination", { from, to, count })
          }
        />
      </Paper>
    </>
  );
}
