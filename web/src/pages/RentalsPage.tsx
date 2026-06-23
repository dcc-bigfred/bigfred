import { useMemo, useState } from "react";
import {
  Box,
  Button,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Tab,
  Tabs,
  Tooltip,
  Typography,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import ReplyIcon from "@mui/icons-material/Reply";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import {
  invalidateLeases,
  useReceivedLeases,
  useRevokeLease,
  type LeaseEntry,
} from "../api/leases";
import { useMe } from "../api/auth";
import GrantedLeasesPanel from "../components/leases/GrantedLeasesPanel";
import LeaseCountdown from "../components/leases/LeaseCountdown";
import { useLeaseEvents } from "../hooks/useLeaseEvents";

export default function RentalsPage() {
  const { t } = useTranslation(["rentals", "common"]);
  const qc = useQueryClient();
  const me = useMe().data;
  const isAdmin = me?.effectiveRole === "admin";
  useLeaseEvents();
  const [tab, setTab] = useState(0);
  const [confirmReturn, setConfirmReturn] = useState<LeaseEntry | null>(null);

  const received = useReceivedLeases();
  const revoke = useRevokeLease();

  const receivedRows = useMemo(() => received.data ?? [], [received.data]);
  const isRefreshingReceived = received.isFetching && !received.isLoading;

  return (
    <Container maxWidth="md" sx={{ py: 3 }}>
      <Typography variant="h5" sx={{ mb: 2 }}>
        {t("title")}
      </Typography>

      <Tabs value={tab} onChange={(_, v: number) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label={t("tabs.received")} />
        <Tab label={t("tabs.granted")} />
      </Tabs>

      {tab === 0 ? (
        <>
          <Stack direction="row" justifyContent="flex-end" sx={{ mb: 2 }}>
            <Button
              variant="outlined"
              startIcon={
                isRefreshingReceived ? (
                  <CircularProgress size={16} color="inherit" />
                ) : (
                  <RefreshIcon />
                )
              }
              onClick={() => void invalidateLeases(qc)}
              disabled={isRefreshingReceived}
            >
              {t("refresh")}
            </Button>
          </Stack>

          {received.isLoading && (
            <Typography color="text.secondary">{t("common:loading")}</Typography>
          )}

          {!received.isLoading && receivedRows.length === 0 && (
            <Typography color="text.secondary">{t("empty")}</Typography>
          )}

          <Stack spacing={1}>
            {receivedRows.map((row) => (
              <Box
                key={`${row.kind}-${row.targetId}`}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 2,
                  p: 1.5,
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography fontWeight={600} noWrap>
                    {row.targetName}
                  </Typography>
                  <Typography variant="body2" color="text.secondary" noWrap>
                    {t("received.from", { login: row.fromLogin })}
                  </Typography>
                </Box>
                <LeaseCountdown expiresAt={row.expiresAt} expiredLabel={t("expired")} />
                <Tooltip title={t("received.return")}>
                  <IconButton
                    color="primary"
                    aria-label={t("received.return")}
                    onClick={() => setConfirmReturn(row)}
                  >
                    <ReplyIcon />
                  </IconButton>
                </Tooltip>
              </Box>
            ))}
          </Stack>
        </>
      ) : (
        <GrantedLeasesPanel allowUnresolvedTarget={isAdmin} />
      )}

      <Dialog open={confirmReturn != null} onClose={() => setConfirmReturn(null)}>
        <DialogTitle>{t("received.returnConfirmTitle")}</DialogTitle>
        <DialogContent>
          <Typography>
            {t("received.returnConfirmBody", {
              name: confirmReturn?.targetName ?? "",
            })}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmReturn(null)}>{t("common:actions.cancel")}</Button>
          <Button
            color="primary"
            variant="contained"
            disabled={revoke.isPending}
            onClick={() => {
              if (!confirmReturn) return;
              void revoke
                .mutateAsync({
                  kind: confirmReturn.kind,
                  targetId: confirmReturn.targetId,
                })
                .then(() => setConfirmReturn(null));
            }}
          >
            {t("received.return")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
