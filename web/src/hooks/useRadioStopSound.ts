import { useEffect, useRef } from "react";

import { useSocket } from "../context/SocketContext";
import radiostopUrl from "../sounds/radiostop.ogg";

// useRadioStopSound plays the radiostop alarm on every system.radioStop
// push event while throttle mode is open (§4.6.3).
export function useRadioStopSound() {
  const { subscribe } = useSocket();
  const audioRef = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    return subscribe("system.radioStop", () => {
      if (!audioRef.current) {
        audioRef.current = new Audio(radiostopUrl);
      }
      void audioRef.current.play().catch(() => {
        // Autoplay may be blocked until the user has gestured once.
      });
    });
  }, [subscribe]);
}
