export type TrainAnnouncementEntry = {
  soundKey: string;
  labelKey: TrainAnnouncementLabelKey;
};

export type TrainAnnouncementLabelKey =
  | "track1Freight"
  | "departureGluszyca"
  | "departureWroclaw"
  | "departureWarszawaCentralna";

/** Interlocking name → ordered announcement list. Use "default" as fallback. */
export const TRAIN_ANNOUNCEMENTS: Record<string, TrainAnnouncementEntry[]> = {
  default: [
    { soundKey: "track-1-freight", labelKey: "track1Freight" },
    { soundKey: "departure-gluszyca", labelKey: "departureGluszyca" },
    { soundKey: "departure-wroclaw", labelKey: "departureWroclaw" },
    {
      soundKey: "departure-warszawa-centralna",
      labelKey: "departureWarszawaCentralna",
    },
  ],
};

export function trainAnnouncementsFor(
  interlockingName: string,
): TrainAnnouncementEntry[] {
  return TRAIN_ANNOUNCEMENTS[interlockingName] ?? TRAIN_ANNOUNCEMENTS.default ?? [];
}
