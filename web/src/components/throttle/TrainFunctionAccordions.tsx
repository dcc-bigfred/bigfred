import { useMemo } from "react";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Chip,
  IconButton,
  Typography,
} from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import SettingsIcon from "@mui/icons-material/Settings";
import { useTranslation } from "react-i18next";

import { useVehicleFunctions } from "../../api/functions";
import type { LocoState } from "../../context/DccBusContext";
import type { ThrottleCockpitFunction } from "./ThrottleCockpit";
import FunctionButtonsPanel from "./FunctionButtonsPanel";
import { cockpit } from "./throttleCockpitTheme";

export interface TrainAccordionMember {
  memberId: number;
  vehicleId: string;
  name: string;
  dccAddress: number | null;
  isDummy: boolean;
  isLeading: boolean;
  speedMultiplier: number;
}

export interface TrainFunctionAccordionsProps {
  members: TrainAccordionMember[];
  states: Map<number, LocoState>;
  expandedMemberIds: number[];
  onToggleExpanded: (memberId: number) => void;
  onFunctionToggle: (memberId: number, fn: number) => void;
  onOpenSettings: (memberId: number) => void;
  showMultiplierCog: boolean;
  functionsAsList?: boolean;
  disabled?: boolean;
}

function useMemberFunctions(vehicleId: string): ThrottleCockpitFunction[] {
  const fnList = useVehicleFunctions(vehicleId).data ?? [];
  return useMemo(
    () =>
      [...fnList]
        .sort((a, b) => a.position - b.position)
        .map((f) => ({ num: f.num, label: f.name, icon: f.icon })),
    [fnList],
  );
}

function TrainDummyMemberRow({ member }: { member: TrainAccordionMember }) {
  const { t } = useTranslation(["throttle", "vehicle"]);

  return (
    <Box
      sx={{
        bgcolor: cockpit.bgPanel,
        color: cockpit.text,
        border: `1px solid ${cockpit.border}`,
        borderRadius: "4px",
        display: "flex",
        alignItems: "center",
        gap: 1,
        px: 1.5,
        py: 1,
        minHeight: 44,
      }}
    >
      <Typography variant="body2" noWrap sx={{ fontWeight: 600, flex: 1, minWidth: 0 }}>
        {member.name}
      </Typography>
      <Chip
        size="small"
        label={t("vehicle:dummyBadge")}
        sx={{
          height: 22,
          fontSize: "0.7rem",
          color: cockpit.textMuted,
          border: `1px solid ${cockpit.border}`,
          "& .MuiChip-label": { px: 1 },
        }}
      />
    </Box>
  );
}

function TrainPoweredMemberAccordion({
  member,
  state,
  expanded,
  onToggleExpanded,
  onFunctionToggle,
  onOpenSettings,
  showMultiplierCog,
  functionsAsList,
  disabled,
}: {
  member: TrainAccordionMember & { dccAddress: number };
  state: LocoState | undefined;
  expanded: boolean;
  onToggleExpanded: () => void;
  onFunctionToggle: (fn: number) => void;
  onOpenSettings: () => void;
  showMultiplierCog: boolean;
  functionsAsList: boolean;
  disabled: boolean;
}) {
  const { t } = useTranslation("throttle");
  const functions = useMemberFunctions(member.vehicleId);
  const functionStates = state?.functions ?? [];

  return (
    <Accordion
      expanded={expanded}
      onChange={onToggleExpanded}
      disableGutters
      sx={{
        bgcolor: cockpit.bgPanel,
        color: cockpit.text,
        "&:before": { display: "none" },
        border: `1px solid ${cockpit.border}`,
        borderRadius: "4px !important",
      }}
    >
      <AccordionSummary
        expandIcon={<ExpandMoreIcon sx={{ color: cockpit.textMuted }} />}
        sx={{ minHeight: 44, "& .MuiAccordionSummary-content": { my: 0.5 } }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            width: "100%",
            gap: 1,
            pr: 1,
          }}
        >
          <Box sx={{ flex: 1, minWidth: 0, display: "flex", alignItems: "baseline", gap: 0.75 }}>
            <Typography variant="body2" noWrap sx={{ fontWeight: 600 }}>
              {member.name}
            </Typography>
            <Typography
              variant="caption"
              component="span"
              aria-label={t("train.memberSpeed", { speed: state?.speed ?? 0 })}
              sx={{
                color: cockpit.textMuted,
                fontSize: "0.65rem",
                fontVariantNumeric: "tabular-nums",
                flexShrink: 0,
              }}
            >
              {state?.speed ?? 0}
            </Typography>
          </Box>
          {member.isLeading && (
            <Chip
              size="small"
              label={t("train.leading")}
              sx={{
                height: 22,
                fontSize: "0.7rem",
                color: "#fff",
                bgcolor: cockpit.btnBottom,
                background: `linear-gradient(180deg, ${cockpit.btnTop} 0%, ${cockpit.btnBottom} 100%)`,
                border: `1px solid ${cockpit.border}`,
                "& .MuiChip-label": { color: "#fff", px: 1 },
              }}
            />
          )}
          {showMultiplierCog && (
            <IconButton
              component="span"
              size="small"
              aria-label={t("train.multiplier.title")}
              onClick={(ev) => {
                ev.stopPropagation();
                onOpenSettings();
              }}
              sx={{ color: cockpit.textMuted }}
            >
              <SettingsIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          pt: 0,
          pb: 1.25,
          px: 1.25,
          bgcolor: "rgb(12, 24, 41)",
        }}
      >
        <FunctionButtonsPanel
          functions={functions}
          asList={functionsAsList}
          isActive={(fnNum) => Boolean(functionStates[fnNum])}
          onToggle={onFunctionToggle}
          disabled={disabled}
        />
      </AccordionDetails>
    </Accordion>
  );
}

export default function TrainFunctionAccordions({
  members,
  states,
  expandedMemberIds,
  onToggleExpanded,
  onFunctionToggle,
  onOpenSettings,
  showMultiplierCog,
  functionsAsList = false,
  disabled = false,
}: TrainFunctionAccordionsProps) {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      {members.map((member) => {
        if (member.isDummy || member.dccAddress == null) {
          return <TrainDummyMemberRow key={member.memberId} member={member} />;
        }
        return (
          <TrainPoweredMemberAccordion
            key={member.memberId}
            member={{ ...member, dccAddress: member.dccAddress }}
            state={states.get(member.dccAddress)}
            expanded={expandedMemberIds.includes(member.memberId)}
            onToggleExpanded={() => onToggleExpanded(member.memberId)}
            onFunctionToggle={(fn) => onFunctionToggle(member.memberId, fn)}
            onOpenSettings={() => onOpenSettings(member.memberId)}
            showMultiplierCog={showMultiplierCog}
            functionsAsList={functionsAsList}
            disabled={disabled}
          />
        );
      })}
    </Box>
  );
}
