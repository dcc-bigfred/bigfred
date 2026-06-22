import { useMemo, useState } from "react";
import {
  Box,
  Button,
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
import ReplyIcon from "@mui/icons-material/Reply";
import HandshakeIcon from "@mui/icons-material/Handshake";
import { useTranslation } from "react-i18next";

import {
  useGrantedLeases,
  useReceivedLeases,
  useRevokeLease,
  type LeaseEntry,
} from "../api/leases";
import { useMe } from "../api/auth";
import LeaseCountdown from "../components/leases/LeaseCountdown";
import LeaseControlDialog from "../components/leases/LeaseControlDialog";
import LeaseCreateDialog from "../components/leases/LeaseCreateDialog";

import { useLeaseEvents } from "../hooks/useLeaseEvents";

export default function RentalsPage() {
  const { t } = useTranslation(["rentals", "common"]);
  const me = useMe().data;
  const isAdmin = me?.effectiveRole === "admin";
  useLeaseEvents();
  const [tab, setTab] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [controlLease, setControlLease] = useState<LeaseEntry | null>(null);
  const [confirmReturn, setConfirmReturn] = useState<LeaseEntry | null>(null);

  const received = useReceivedLeases();
  const granted = useGrantedLeases();
  const revoke = useRevokeLease();

  const rows = useMemo(
    () => (tab === 0 ? received.data ?? [] : granted.data ?? []),
    [tab, received.data, granted.data],
  );

  const loading = tab === 0 ? received.isLoading : granted.isLoading;

  return (
    <Container maxWidth="md" sx={{ py: 3 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
        <Typography variant="h5">{t("title")}</Typography>
        {tab === 1 && (
          <Button variant="contained" startIcon={<HandshakeIcon />} onClick={() => setCreateOpen(true)}>
            {t("granted.lend")}
          </Button>
        )}
      </Stack>

      <Tabs value={tab} onChange={(_, v: number) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label={t("tabs.received")} />
        <Tab label={t("tabs.granted")} />
      </Tabs>

      {loading && <Typography color="text.secondary">{t("common:loading")}</Typography>}

      {!loading && rows.length === 0 && (
        <Typography color="text.secondary">{t("empty")}</Typography>
      )}

      <Stack spacing={1}>
        {rows.map((row) => (
          <Box
            key={`${row.kind}-${row.targetId}`}
            onClick={tab === 1 ? () => setControlLease(row) : undefined}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              p: 1.5,
              borderRadius: 1,
              border: 1,
              borderColor: "divider",
              cursor: tab === 1 ? "pointer" : "default",
              "&:hover": tab === 1 ? { bgcolor: "action.hover" } : undefined,
            }}
          >
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography fontWeight={600} noWrap>
                {row.targetName}
              </Typography>
              <Typography variant="body2" color="text.secondary" noWrap>
                {tab === 0
                  ? t("received.from", { login: row.fromLogin })
                  : t("granted.to", { login: row.toLogin })}
              </Typography>
            </Box>
            <LeaseCountdown expiresAt={row.expiresAt} expiredLabel={t("expired")} />
            {tab === 0 && (
              <Tooltip title={t("received.return")}>
                <IconButton
                  color="primary"
                  aria-label={t("received.return")}
                  onClick={() => setConfirmReturn(row)}
                >
                  <ReplyIcon />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        ))}
      </Stack>

      <LeaseCreateDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        allowUnresolvedTarget={isAdmin}
      />
      <LeaseControlDialog
        lease={controlLease}
        open={controlLease != null}
        onClose={() => setControlLease(null)}
      />

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
