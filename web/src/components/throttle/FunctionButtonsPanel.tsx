import { Box } from "@mui/material";
import { useTranslation } from "react-i18next";

import type { ThrottleCockpitFunction } from "./ThrottleCockpit";
import FunctionGridButton from "./FunctionGridButton";
import FunctionListRow from "./FunctionListRow";
import {
  FUNCTION_BUTTON_GRID_GAP_PX,
  FUNCTION_BUTTON_SIZE_PX,
  FUNCTION_LIST_ROW_GAP_PX,
} from "./throttleCockpitTheme";

export interface FunctionButtonsPanelProps {
  functions: ThrottleCockpitFunction[];
  asList: boolean;
  isActive: (fnNum: number) => boolean;
  onToggle: (fn: number) => void;
  disabled?: boolean;
}

/** Shared function grid / list renderer for vehicle and train throttle modes. */
export default function FunctionButtonsPanel({
  functions,
  asList,
  isActive,
  onToggle,
  disabled = false,
}: FunctionButtonsPanelProps) {
  const { t } = useTranslation("throttle");

  if (asList) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: `${FUNCTION_LIST_ROW_GAP_PX}px`,
          width: "100%",
        }}
      >
        {functions.map((fn) => (
          <FunctionListRow
            key={fn.num}
            fnCode={t("fnLabel", { n: fn.num })}
            fnNum={fn.num}
            label={fn.label}
            icon={fn.icon}
            active={isActive(fn.num)}
            disabled={disabled}
            onToggle={onToggle}
          />
        ))}
      </Box>
    );
  }

  return (
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
          fnNum={fn.num}
          label={fn.label}
          icon={fn.icon}
          active={isActive(fn.num)}
          disabled={disabled}
          onToggle={onToggle}
        />
      ))}
    </Box>
  );
}
