import { useCallback, useRef } from "react";

import radioSentUrl from "../sounds/interlockings/radio-sent.ogg";

// useRadioInboundSound plays the inbound radio chime (radio-sent.ogg).
export function useRadioInboundSound() {
  const audioRef = useRef<HTMLAudioElement | null>(null);

  return useCallback(() => {
    if (!audioRef.current) {
      audioRef.current = new Audio(radioSentUrl);
    }
    audioRef.current.currentTime = 0;
    void audioRef.current.play().catch(() => {});
  }, []);
}
