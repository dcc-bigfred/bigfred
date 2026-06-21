import { type ReactNode } from "react";
import { Stack } from "@mui/material";

import type { DriverRadioInbound } from "./useDriverRadioInbound";
import ThrottleChatButton from "../components/throttle/ThrottleChatButton";
import ThrottleRadioButton from "../components/throttle/ThrottleRadioButton";

interface ThrottleRadioHeaderArgs {
  layoutId: number;
  vehicleId: string | null;
  vehicleName: string | null;
  radio: DriverRadioInbound;
}

// buildThrottleRadioHeader renders radio + chat icons for the cockpit bar.
export function buildThrottleRadioHeader({
  layoutId,
  vehicleId,
  vehicleName,
  radio,
}: ThrottleRadioHeaderArgs): ReactNode {
  return (
    <Stack direction="row" spacing={0.25} alignItems="center">
      <ThrottleRadioButton
        layoutId={layoutId}
        vehicleId={vehicleId}
        vehicleName={vehicleName}
      />
      <ThrottleChatButton
        unreadCount={radio.unreadCount}
        onOpen={radio.openChat}
      />
    </Stack>
  );
}
