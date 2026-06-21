import { useCallback, useEffect, useState } from "react";

function storageKey(trainId: string): string {
  return `bigfred.throttle.train.${trainId}.expanded`;
}

function readExpanded(trainId: string): number[] {
  try {
    const raw = sessionStorage.getItem(storageKey(trainId));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((id): id is number => typeof id === "number");
  } catch {
    return [];
  }
}

function writeExpanded(trainId: string, memberIds: number[]) {
  try {
    sessionStorage.setItem(storageKey(trainId), JSON.stringify(memberIds));
  } catch {
    /* ignore */
  }
}

/** Remembers which train-member function accordions are expanded. */
export function useTrainAccordionExpanded(trainId: string | null) {
  const [expandedMemberIds, setExpandedMemberIds] = useState<number[]>([]);

  useEffect(() => {
    if (trainId == null) {
      setExpandedMemberIds([]);
      return;
    }
    setExpandedMemberIds(readExpanded(trainId));
  }, [trainId]);

  const setExpanded = useCallback(
    (memberIds: number[]) => {
      setExpandedMemberIds(memberIds);
      if (trainId != null) {
        writeExpanded(trainId, memberIds);
      }
    },
    [trainId],
  );

  const toggleMember = useCallback(
    (memberId: number) => {
      setExpandedMemberIds((prev) => {
        const next = prev.includes(memberId)
          ? prev.filter((id) => id !== memberId)
          : [...prev, memberId];
        if (trainId != null) {
          writeExpanded(trainId, next);
        }
        return next;
      });
    },
    [trainId],
  );

  return { expandedMemberIds, setExpanded, toggleMember };
}
