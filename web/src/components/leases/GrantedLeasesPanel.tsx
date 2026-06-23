import { useState } from "react";
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  Typography,
} from "@mui/material";
import HandshakeIcon from "@mui/icons-material/Handshake";
import RefreshIcon from "@mui/icons-material/Refresh";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { invalidateLeases, useGrantedLeases, type LeaseEntry } from "../../api/leases";
import { useLeaseEvents } from "../../hooks/useLeaseEvents";
import LeaseCountdown from "./LeaseCountdown";
import LeaseControlDialog from "./LeaseControlDialog";
import LeaseCreateDialog from "./LeaseCreateDialog";

export interface GrantedLeasesPanelProps {
  /** Pre-fill lend dialog targets not in the caller's lendable list (admin). */
  allowUnresolvedTarget?: boolean;
  /** Show the vehicle/train owner alongside the lessee (layout-wide admin view). */
  showOwner?: boolean;
}

export default function GrantedLeasesPanel({
  allowUnresolvedTarget = false,
  showOwner = false,
}: GrantedLeasesPanelProps) {
  const { t } = useTranslation(["rentals", "common"]);
  const qc = useQueryClient();
  useLeaseEvents();

  const granted = useGrantedLeases();
  const isRefreshing = granted.isFetching && !granted.isLoading;
  const rows = granted.data ?? [];

  const [createOpen, setCreateOpen] = useState(false);
  const [controlLease, setControlLease] = useState<LeaseEntry | null>(null);

  const renderSubtitle = (row: LeaseEntry) => {
    if (showOwner) {
      return `${t("received.from", { login: row.fromLogin })} · ${t("granted.to", { login: row.toLogin })}`;
    }
    return t("granted.to", { login: row.toLogin });
  };

  return (
    <>
      <Stack direction="row" alignItems="center" justifyContent="flex-end" spacing={1} sx={{ mb: 2 }}>
        <Button
          variant="outlined"
          startIcon={
            isRefreshing ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <RefreshIcon />
            )
          }
          onClick={() => void invalidateLeases(qc)}
          disabled={isRefreshing}
        >
          {t("refresh")}
        </Button>
        <Button
          variant="contained"
          startIcon={<HandshakeIcon />}
          onClick={() => setCreateOpen(true)}
        >
          {t("granted.lend")}
        </Button>
      </Stack>

      {granted.isLoading && (
        <Typography color="text.secondary">{t("common:loading")}</Typography>
      )}

      {!granted.isLoading && rows.length === 0 && (
        <Typography color="text.secondary">{t("empty")}</Typography>
      )}

      <Stack spacing={1}>
        {rows.map((row) => (
          <Box
            key={`${row.kind}-${row.targetId}`}
            onClick={() => setControlLease(row)}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              p: 1.5,
              borderRadius: 1,
              border: 1,
              borderColor: "divider",
              cursor: "pointer",
              "&:hover": { bgcolor: "action.hover" },
            }}
          >
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography fontWeight={600} noWrap>
                {row.targetName}
              </Typography>
              <Typography variant="body2" color="text.secondary" noWrap>
                {renderSubtitle(row)}
              </Typography>
            </Box>
            <LeaseCountdown expiresAt={row.expiresAt} expiredLabel={t("expired")} />
          </Box>
        ))}
      </Stack>

      <LeaseCreateDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        allowUnresolvedTarget={allowUnresolvedTarget}
      />
      <LeaseControlDialog
        lease={controlLease}
        open={controlLease != null}
        onClose={() => setControlLease(null)}
      />
    </>
  );
}
