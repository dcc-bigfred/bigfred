import { useCallback, useEffect, useState } from "react";

export interface ConnectedGamepad {
  index: number;
  id: string;
  buttonCount: number;
  axisCount: number;
}

function readConnectedGamepads(): ConnectedGamepad[] {
  const pads = navigator.getGamepads?.() ?? [];
  const result: ConnectedGamepad[] = [];
  for (const gp of pads) {
    if (!gp) continue;
    result.push({
      index: gp.index,
      id: gp.id,
      buttonCount: gp.buttons.length,
      axisCount: gp.axes.length,
    });
  }
  return result;
}

/** Tracks gamepads visible to the page via the Gamepad API. */
export function useGamepads() {
  const [gamepads, setGamepads] = useState<ConnectedGamepad[]>(() =>
    readConnectedGamepads(),
  );

  const refresh = useCallback(() => {
    setGamepads(readConnectedGamepads());
  }, []);

  useEffect(() => {
    const onConnect = () => refresh();
    const onDisconnect = () => refresh();
    window.addEventListener("gamepadconnected", onConnect);
    window.addEventListener("gamepaddisconnected", onDisconnect);
    refresh();
    return () => {
      window.removeEventListener("gamepadconnected", onConnect);
      window.removeEventListener("gamepaddisconnected", onDisconnect);
    };
  }, [refresh]);

  return { gamepads, refresh };
}
