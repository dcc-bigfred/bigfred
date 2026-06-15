import { useCallback, useEffect, useRef } from "react";

import type { RadioPhrase } from "../api/radio";
import { useSocket } from "../context/SocketContext";
import radioSentUrl from "../sounds/interlockings/radio-sent.ogg";

function phraseSoundUrl(phrase: RadioPhrase): string {
  const file = phrase.toLowerCase().replace(/_/g, "-");
  return `/sounds/interlockings/${file}.ogg`;
}

// useRadioSounds plays radio-sent on outbound ack and phrase audio (or
// radio-sent fallback) on inbound radio.message while mounted.
export function useRadioSounds(enabled = true) {
  const { subscribe } = useSocket();
  const sentRef = useRef<HTMLAudioElement | null>(null);
  const cacheRef = useRef(new Map<string, HTMLAudioElement>());

  const playSent = useCallback(() => {
    if (!enabled) return;
    if (!sentRef.current) {
      sentRef.current = new Audio(radioSentUrl);
    }
    void sentRef.current.play().catch(() => {});
  }, [enabled]);

  const playPhrase = useCallback(
    (phrase: RadioPhrase) => {
      if (!enabled) return;
      const url = phraseSoundUrl(phrase);
      let audio = cacheRef.current.get(url);
      if (!audio) {
        audio = new Audio(url);
        cacheRef.current.set(url, audio);
      }
      audio.onerror = () => {
        if (!sentRef.current) {
          sentRef.current = new Audio(radioSentUrl);
        }
        void sentRef.current.play().catch(() => {});
      };
      audio.currentTime = 0;
      void audio.play().catch(() => {
        if (!sentRef.current) {
          sentRef.current = new Audio(radioSentUrl);
        }
        void sentRef.current.play().catch(() => {});
      });
    },
    [enabled],
  );

  useEffect(() => {
    if (!enabled) return;
    return subscribe("radio.message", (payload) => {
      const msg = payload as { phrase?: RadioPhrase };
      if (msg.phrase) {
        playPhrase(msg.phrase);
      }
    });
  }, [enabled, subscribe, playPhrase]);

  return { playSent, playPhrase };
}
