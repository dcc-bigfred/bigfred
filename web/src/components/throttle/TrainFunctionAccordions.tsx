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
import FunctionGridButton from "./FunctionGridButton";
import {
  cockpit,
  FUNCTION_BUTTON_GRID_GAP_PX,
  FUNCTION_BUTTON_SIZE_PX,
} from "./throttleCockpitTheme";

export interface TrainAccordionMember {
  memberId: number;
  vehicleId: number;
  name: string;
  dccAddress: number;
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
  disabled?: boolean;
}

function useMemberFunctions(vehicleId: number): ThrottleCockpitFunction[] {
  const fnList = useVehicleFunctions(vehicleId).data ?? [];
  return useMemo(
    () =>
      [...fnList]
        .sort((a, b) => a.position - b.position)
        .map((f) => ({ num: f.num, label: f.name, icon: f.icon })),
    [fnList],
  );
}

function TrainMemberAccordion({
  member,
  state,
  expanded,
  onToggleExpanded,
  onFunctionToggle,
  onOpenSettings,
  showMultiplierCog,
  disabled,
}: {
  member: TrainAccordionMember;
  state: LocoState | undefined;
  expanded: boolean;
  onToggleExpanded: () => void;
  onFunctionToggle: (fn: number) => void;
  onOpenSettings: () => void;
  showMultiplierCog: boolean;
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
          <Typography variant="body2" sx={{ flex: 1, fontWeight: 600 }}>
            {member.name}
          </Typography>
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
          {showMultiplierCog && !member.isLeading && (
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
        <Box
          sx={{
            display: "grid",
            width: "100%",
            gridTemplateColumns: `repeat(auto-fill, ${FUNCTION_BUTTON_SIZE_PX}px)`,
            gap: `${FUNCTION_BUTTON_GRID_GAP_PX}px`,
            justifyContent: "start",
          }}
        >
          {functions.map((fn) => (
            <FunctionGridButton
              key={fn.num}
              fnCode={t("fnLabel", { n: fn.num })}
              label={fn.label}
              icon={fn.icon}
              active={Boolean(functionStates[fn.num])}
              disabled={disabled}
              onClick={() => onFunctionToggle(fn.num)}
            />
          ))}
        </Box>
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
  disabled = false,
}: TrainFunctionAccordionsProps) {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      {members.map((member) => (
        <TrainMemberAccordion
          key={member.memberId}
          member={member}
          state={states.get(member.dccAddress)}
          expanded={expandedMemberIds.includes(member.memberId)}
          onToggleExpanded={() => onToggleExpanded(member.memberId)}
          onFunctionToggle={(fn) => onFunctionToggle(member.memberId, fn)}
          onOpenSettings={() => onOpenSettings(member.memberId)}
          showMultiplierCog={showMultiplierCog}
          disabled={disabled}
        />
      ))}
    </Box>
  );
}
