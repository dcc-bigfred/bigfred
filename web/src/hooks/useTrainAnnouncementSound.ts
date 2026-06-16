import { useCallback, useRef, useState } from "react";

function announcementSoundUrl(soundKey: string): string {
  return `/sounds/train-announcements/${soundKey}.ogg`;
}

// useTrainAnnouncementSound plays a local PA clip on the clicking device.
export function useTrainAnnouncementSound() {
  const cacheRef = useRef(new Map<string, HTMLAudioElement>());
  const currentRef = useRef<HTMLAudioElement | null>(null);
  const [playingSoundKey, setPlayingSoundKey] = useState<string | null>(null);

  const play = useCallback((soundKey: string) => {
    const url = announcementSoundUrl(soundKey);
    let audio = cacheRef.current.get(url);
    if (!audio) {
      audio = new Audio(url);
      cacheRef.current.set(url, audio);
      audio.addEventListener("ended", () => {
        if (currentRef.current === audio) {
          setPlayingSoundKey(null);
        }
      });
    }

    if (currentRef.current && currentRef.current !== audio) {
      currentRef.current.pause();
      currentRef.current.currentTime = 0;
    }

    currentRef.current = audio;
    audio.currentTime = 0;
    setPlayingSoundKey(soundKey);
    void audio.play().catch(() => {
      if (currentRef.current === audio) {
        setPlayingSoundKey(null);
      }
    });
  }, []);

  return { play, playingSoundKey };
}
