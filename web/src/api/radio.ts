import { useCallback, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";
import { getUserName } from "../utils/getUserName";

// Driver phrases — ordered: acks/entry/departure, then shunting, then other.
export const RADIO_PHRASES_DRIVER = [
  "ACK",
  "STOPPED_AT_SIGNAL_READY_TO_ENTER",
  "READY_TO_DEPART",
  "ACCEPTED_DEPARTURE_ON_REPLACEMENT_SIGNAL",
  "ACCEPTED_HELPER_DETACH_AT_STATION",
  "ACCEPTED_WAITING_FOR_OPPOSITE_TRAIN",
  "TRAIN_ARRIVED_COMPLETE_AT_STATION",
  "LOCO_READY_FOR_RUN_AROUND",
  "ACCEPTED_CROSSINGS_EXTRA_CAUTION",
  "ACCEPTED_PUSHING_BEYOND_POINTS",
  "ACCEPTED_STOPPING_SHUNTING",
  "PASSED_POINTS_AHEAD_WAITING_RUN_AROUND_ROUTE",
  "REACHED_POINTS_REAR_WAITING_RUN_AROUND_ROUTE",
  "READY_TO_COUPLE_WAGONS",
  "READY_TO_UNCOUPLE_WAGONS",
  "REPORTING_CONSIST_COUPLED_TO_LOCO",
  "WAGONS_TAKEN",
  "WAGONS_SET_ASIDE",
  "RADIO_LINK_RESTORED",
  "LEVEL_CROSSING_GATES_OPEN",
] as const;

// Signalman phrases — ordered: acks/entry/departure, then shunting, then other.
export const RADIO_PHRASES_SIGNALMAN = [
  "ACK",
  "AGREED",
  "REPORT_ACKNOWLEDGED",
  "REPETITION_CORRECT",
  "CONFIRMED",
  "NO",
  "REFUSED_WAIT_FOR_SIGNAL",
  "ENTRY_PERMITTED",
  "DEPARTURE_CLEARED",
  "DEPARTURE_ON_REPLACEMENT_SIGNAL",
  "ROUTE_SET",
  "ARRIVAL_COMPLETE_ACKNOWLEDGED",
  "TRAIN_TRACK_1_FREE_RECEIVE_TRACK_1",
  "TRAIN_TRACK_2_FREE_RECEIVE_TRACK_2",
  "TRAIN_TRACK_3_FREE_RECEIVE_TRACK_3",
  "TRAIN_TRACK_4_FREE_RECEIVE_TRACK_4",
  "TRAIN_TRACK_5_FREE_RECEIVE_TRACK_5",
  "TRAIN_TRACK_6_FREE_RECEIVE_TRACK_6",
  "TRAIN_TRACK_7_FREE_RECEIVE_TRACK_7",
  "TRAIN_TRACK_8_FREE_RECEIVE_TRACK_8",
  "WRONG_ROAD_FROM_POST_TO_STATION",
  "ACCEPTED_WRONG_ROAD_FROM_POST_TO_STATION",
  "CANCEL_ROUTE",
  "SHUNTING_EXTRA_CAUTION_THROUGH_POINTS",
  "RUN_AROUND_PERMITTED",
  "PUSHING_BEYOND_POINTS_PERMITTED",
  "STOP_SHUNTING_IMMEDIATELY",
  "HELPER_LOCO_WILL_DETACH_AT_STATION",
  "STOP_IMMEDIATELY",
  "ACCEPTED_NOTIFYING_GATEKEEPER_AND_NEIGHBORS",
  "TRAIN_WAITING_FOR_OPPOSITE",
  "STAFF_ON_TRACK_CAUTION_SIGNAL",
] as const;

export type RadioPhrase =
  | (typeof RADIO_PHRASES_DRIVER)[number]
  | (typeof RADIO_PHRASES_SIGNALMAN)[number];

export type RadioPhraseSide = "driver" | "signalman";

export type RadioPhraseGroup = "shunting" | "arrival" | "departure" | "confirmations" | "other";

export const RADIO_PHRASE_GROUP_ALL = "all" as const;

export type RadioPhraseGroupFilter = RadioPhraseGroup | typeof RADIO_PHRASE_GROUP_ALL;

export const RADIO_PHRASE_GROUP_ORDER: readonly RadioPhraseGroup[] = [
  "shunting",
  "arrival",
  "departure",
  "confirmations",
  "other",
] as const;

const DRIVER_PHRASE_GROUP: Record<(typeof RADIO_PHRASES_DRIVER)[number], RadioPhraseGroup> = {
  ACK: "confirmations",
  STOPPED_AT_SIGNAL_READY_TO_ENTER: "arrival",
  READY_TO_DEPART: "departure",
  ACCEPTED_DEPARTURE_ON_REPLACEMENT_SIGNAL: "departure",
  ACCEPTED_HELPER_DETACH_AT_STATION: "shunting",
  ACCEPTED_WAITING_FOR_OPPOSITE_TRAIN: "arrival",
  TRAIN_ARRIVED_COMPLETE_AT_STATION: "arrival",
  LOCO_READY_FOR_RUN_AROUND: "shunting",
  ACCEPTED_CROSSINGS_EXTRA_CAUTION: "shunting",
  ACCEPTED_PUSHING_BEYOND_POINTS: "shunting",
  ACCEPTED_STOPPING_SHUNTING: "shunting",
  PASSED_POINTS_AHEAD_WAITING_RUN_AROUND_ROUTE: "shunting",
  REACHED_POINTS_REAR_WAITING_RUN_AROUND_ROUTE: "shunting",
  READY_TO_COUPLE_WAGONS: "shunting",
  READY_TO_UNCOUPLE_WAGONS: "shunting",
  REPORTING_CONSIST_COUPLED_TO_LOCO: "shunting",
  WAGONS_TAKEN: "shunting",
  WAGONS_SET_ASIDE: "shunting",
  RADIO_LINK_RESTORED: "other",
  LEVEL_CROSSING_GATES_OPEN: "other",
};

const DRIVER_PHRASE_CONTEXTUAL_CONFIRMATION = new Set<
  (typeof RADIO_PHRASES_DRIVER)[number]
>([
  "ACCEPTED_DEPARTURE_ON_REPLACEMENT_SIGNAL",
  "ACCEPTED_HELPER_DETACH_AT_STATION",
  "ACCEPTED_WAITING_FOR_OPPOSITE_TRAIN",
  "ACCEPTED_CROSSINGS_EXTRA_CAUTION",
  "ACCEPTED_PUSHING_BEYOND_POINTS",
  "ACCEPTED_STOPPING_SHUNTING",
]);

const SIGNALMAN_PHRASE_GROUP: Record<
  (typeof RADIO_PHRASES_SIGNALMAN)[number],
  RadioPhraseGroup
> = {
  ACK: "confirmations",
  AGREED: "confirmations",
  REPORT_ACKNOWLEDGED: "confirmations",
  REPETITION_CORRECT: "confirmations",
  CONFIRMED: "confirmations",
  NO: "confirmations",
  REFUSED_WAIT_FOR_SIGNAL: "confirmations",
  ENTRY_PERMITTED: "arrival",
  DEPARTURE_CLEARED: "departure",
  DEPARTURE_ON_REPLACEMENT_SIGNAL: "departure",
  ROUTE_SET: "departure",
  ARRIVAL_COMPLETE_ACKNOWLEDGED: "arrival",
  TRAIN_TRACK_1_FREE_RECEIVE_TRACK_1: "arrival",
  TRAIN_TRACK_2_FREE_RECEIVE_TRACK_2: "arrival",
  TRAIN_TRACK_3_FREE_RECEIVE_TRACK_3: "arrival",
  TRAIN_TRACK_4_FREE_RECEIVE_TRACK_4: "arrival",
  TRAIN_TRACK_5_FREE_RECEIVE_TRACK_5: "arrival",
  TRAIN_TRACK_6_FREE_RECEIVE_TRACK_6: "arrival",
  TRAIN_TRACK_7_FREE_RECEIVE_TRACK_7: "arrival",
  TRAIN_TRACK_8_FREE_RECEIVE_TRACK_8: "arrival",
  WRONG_ROAD_FROM_POST_TO_STATION: "arrival",
  ACCEPTED_WRONG_ROAD_FROM_POST_TO_STATION: "arrival",
  CANCEL_ROUTE: "shunting",
  SHUNTING_EXTRA_CAUTION_THROUGH_POINTS: "shunting",
  RUN_AROUND_PERMITTED: "shunting",
  PUSHING_BEYOND_POINTS_PERMITTED: "shunting",
  STOP_SHUNTING_IMMEDIATELY: "shunting",
  HELPER_LOCO_WILL_DETACH_AT_STATION: "shunting",
  STOP_IMMEDIATELY: "shunting",
  ACCEPTED_NOTIFYING_GATEKEEPER_AND_NEIGHBORS: "arrival",
  TRAIN_WAITING_FOR_OPPOSITE: "arrival",
  STAFF_ON_TRACK_CAUTION_SIGNAL: "arrival",
};

const SIGNALMAN_PHRASE_CONTEXTUAL_CONFIRMATION = new Set<
  (typeof RADIO_PHRASES_SIGNALMAN)[number]
>([
  "ARRIVAL_COMPLETE_ACKNOWLEDGED",
  "ACCEPTED_WRONG_ROAD_FROM_POST_TO_STATION",
  "ACCEPTED_NOTIFYING_GATEKEEPER_AND_NEIGHBORS",
]);

export function radioPhrasesForSide(side: RadioPhraseSide): readonly RadioPhrase[] {
  return side === "driver" ? RADIO_PHRASES_DRIVER : RADIO_PHRASES_SIGNALMAN;
}

export function radioPhraseGroup(phrase: RadioPhrase, side: RadioPhraseSide): RadioPhraseGroup {
  if (side === "driver") {
    return DRIVER_PHRASE_GROUP[phrase as (typeof RADIO_PHRASES_DRIVER)[number]];
  }
  return SIGNALMAN_PHRASE_GROUP[phrase as (typeof RADIO_PHRASES_SIGNALMAN)[number]];
}

// radioPhraseIsContextualConfirmation reports whether a phrase is a
// domain-specific ack (e.g. accepted shunting) rather than a general one.
export function radioPhraseIsContextualConfirmation(
  phrase: RadioPhrase,
  side: RadioPhraseSide,
): boolean {
  if (side === "driver") {
    return DRIVER_PHRASE_CONTEXTUAL_CONFIRMATION.has(
      phrase as (typeof RADIO_PHRASES_DRIVER)[number],
    );
  }
  return SIGNALMAN_PHRASE_CONTEXTUAL_CONFIRMATION.has(
    phrase as (typeof RADIO_PHRASES_SIGNALMAN)[number],
  );
}

const OPERATIONAL_RADIO_PHRASE_GROUPS = new Set<RadioPhraseGroup>([
  "shunting",
  "arrival",
  "departure",
]);

// sortRadioPhrasesWithinGroup puts contextual confirmations first inside
// shunting / arrival / departure lists; other groups keep vocabulary order.
export function sortRadioPhrasesWithinGroup(
  phrases: readonly RadioPhrase[],
  side: RadioPhraseSide,
  group: RadioPhraseGroup,
): RadioPhrase[] {
  if (!OPERATIONAL_RADIO_PHRASE_GROUPS.has(group)) {
    return [...phrases];
  }
  const confirmations: RadioPhrase[] = [];
  const operative: RadioPhrase[] = [];
  for (const phrase of phrases) {
    if (radioPhraseIsContextualConfirmation(phrase, side)) {
      confirmations.push(phrase);
    } else {
      operative.push(phrase);
    }
  }
  return [...confirmations, ...operative];
}

const RADIO_PHRASE_GROUP_STORAGE_PREFIX = "bigfred.radio.phrase-group";

const VALID_RADIO_PHRASE_GROUP_FILTERS = new Set<RadioPhraseGroupFilter>([
  RADIO_PHRASE_GROUP_ALL,
  ...RADIO_PHRASE_GROUP_ORDER,
]);

function radioPhraseGroupStorageKey(side: RadioPhraseSide): string {
  return `${RADIO_PHRASE_GROUP_STORAGE_PREFIX}.${side}`;
}

export function readStoredRadioPhraseGroupFilter(side: RadioPhraseSide): RadioPhraseGroupFilter {
  try {
    const raw = sessionStorage.getItem(radioPhraseGroupStorageKey(side));
    if (raw != null && VALID_RADIO_PHRASE_GROUP_FILTERS.has(raw as RadioPhraseGroupFilter)) {
      return raw as RadioPhraseGroupFilter;
    }
  } catch {
    /* ignore */
  }
  return RADIO_PHRASE_GROUP_ALL;
}

export function writeStoredRadioPhraseGroupFilter(
  side: RadioPhraseSide,
  group: RadioPhraseGroupFilter,
): void {
  try {
    sessionStorage.setItem(radioPhraseGroupStorageKey(side), group);
  } catch {
    /* ignore */
  }
}

export interface RadioUser {
  userId: number;
  login: string;
  organization: string;
}

export interface RadioTarget {
  userId?: number;
  interlockingId?: number;
}

export interface RadioContextEntity {
  id: string;
  name: string;
}

export interface RadioInterlockingEntity {
  id: number;
  name: string;
}

export interface RadioContext {
  vehicle?: RadioContextEntity;
  train?: RadioContextEntity;
}

export interface RadioMessage {
  messageId: string;
  from: RadioUser;
  fromInterlocking?: RadioInterlockingEntity;
  to: RadioTarget;
  context: RadioContext;
  phrase: RadioPhrase;
  note?: string;
  sentAt: number;
}

export interface RadioSendTarget {
  userId?: number;
  interlockingId?: number;
}

export interface RadioSendContext {
  vehicleId?: string;
  trainId?: string;
}

function interlockingRadioKey(id: number) {
  return ["interlockings", id, "radio"] as const;
}

function myRadioKey() {
  return ["radio", "mine"] as const;
}

function sortMessages(rows: RadioMessage[]): RadioMessage[] {
  return [...rows].sort((a, b) => a.sentAt - b.sentAt);
}

function mergeMessage(rows: RadioMessage[], msg: RadioMessage): RadioMessage[] {
  if (rows.some((r) => r.messageId === msg.messageId)) {
    return rows;
  }
  return sortMessages([...rows, msg]);
}

function wireToMessages(payload: unknown): RadioMessage[] {
  const data = payload as { messages?: RadioMessage[] };
  return sortMessages(data.messages ?? []);
}

// useInterlockingRadio seeds the signalman group chat from REST and
// keeps it live via radio.message / radio.history events.
export function useInterlockingRadio(interlockingId: number | null) {
  const qc = useQueryClient();
  const { subscribe, connected, sendAction } = useSocket();
  const replayingRef = useRef(false);

  const query = useQuery({
    queryKey: interlockingRadioKey(interlockingId ?? 0),
    queryFn: async () => {
      const data = await apiFetch<{ messages: RadioMessage[] }>(
        `/api/v1/interlockings/${interlockingId}/radio`,
      );
      return sortMessages(data.messages ?? []);
    },
    enabled: interlockingId != null && interlockingId > 0,
    staleTime: 0,
  });

  useEffect(() => {
    if (interlockingId == null || interlockingId <= 0) return;
    return subscribe("radio.message", (payload) => {
      const msg = payload as RadioMessage;
      qc.setQueryData<RadioMessage[]>(interlockingRadioKey(interlockingId), (prev) =>
        mergeMessage(prev ?? [], msg),
      );
    });
  }, [interlockingId, subscribe, qc]);

  useEffect(() => {
    if (interlockingId == null || interlockingId <= 0) return;
    return subscribe("radio.history", (payload) => {
      if (!replayingRef.current) return;
      qc.setQueryData<RadioMessage[]>(
        interlockingRadioKey(interlockingId),
        wireToMessages(payload),
      );
      replayingRef.current = false;
    });
  }, [interlockingId, subscribe, qc]);

  useEffect(() => {
    if (!connected || interlockingId == null || interlockingId <= 0) return;
    replayingRef.current = true;
    void sendAction("radio.replay", {
      scope: "interlocking",
      interlockingId,
    });
  }, [connected, interlockingId, sendAction]);

  return query;
}

// useMyRadio loads the driver's personal radio history.
export function useMyRadio() {
  const qc = useQueryClient();
  const { subscribe, connected, sendAction } = useSocket();
  const replayingRef = useRef(false);

  const query = useQuery({
    queryKey: myRadioKey(),
    queryFn: async () => {
      const data = await apiFetch<{ messages: RadioMessage[] }>("/api/v1/radio/mine");
      return sortMessages(data.messages ?? []);
    },
    staleTime: 0,
  });

  useEffect(() => {
    return subscribe("radio.message", (payload) => {
      const msg = payload as RadioMessage;
      qc.setQueryData<RadioMessage[]>(myRadioKey(), (prev) =>
        mergeMessage(prev ?? [], msg),
      );
    });
  }, [subscribe, qc]);

  useEffect(() => {
    return subscribe("radio.history", (payload) => {
      if (!replayingRef.current) return;
      qc.setQueryData<RadioMessage[]>(myRadioKey(), wireToMessages(payload));
      replayingRef.current = false;
    });
  }, [subscribe, qc]);

  useEffect(() => {
    if (!connected) return;
    replayingRef.current = true;
    void sendAction("radio.replay", { scope: "user" });
  }, [connected, sendAction]);

  return query;
}

export function useSendRadio() {
  const { sendAction } = useSocket();

  return useCallback(
    async (args: {
      to: RadioSendTarget;
      context: RadioSendContext;
      phrase: RadioPhrase;
      note?: string;
    }) => {
      return sendAction("radio.send", {
        to: {
          userId: args.to.userId,
          interlockingId: args.to.interlockingId,
        },
        context: {
          vehicleId: args.context.vehicleId,
          trainId: args.context.trainId,
        },
        phrase: args.phrase,
        note: args.note ?? "",
      });
    },
    [sendAction],
  );
}

export function contextLabel(msg: RadioMessage): string {
  if (msg.context.vehicle) {
    return msg.context.vehicle.name;
  }
  if (msg.context.train) {
    return msg.context.train.name;
  }
  return "";
}

// driverChatInterlockingLabel returns the interlocking name for a line in
// the driver's personal radio history (inbound fromInterlocking or outbound
// to.interlockingId resolved against the layout catalogue).
export function driverChatInterlockingLabel(
  msg: RadioMessage,
  interlockingNames: ReadonlyMap<number, string>,
): string {
  if (msg.fromInterlocking?.name) {
    return msg.fromInterlocking.name;
  }
  const toId = msg.to.interlockingId;
  if (toId != null) {
    return interlockingNames.get(toId) ?? "";
  }
  return "";
}

export function radioSenderLabel(msg: RadioMessage): string {
  if (msg.fromInterlocking?.name) {
    return msg.fromInterlocking.name;
  }
  return getUserName(msg.from);
}

export function formatRadioAlertLine(msg: RadioMessage, phraseLabel: string): string {
  const message = msg.note?.trim()
    ? `${phraseLabel} — ${msg.note.trim()}`
    : phraseLabel;
  return `${radioSenderLabel(msg)}: ${message}`;
}

// formatRadioMessageTime renders sentAt as HH:mm:ss in local time.
export function formatRadioMessageTime(sentAtMs: number): string {
  const d = new Date(sentAtMs);
  const hh = d.getHours().toString().padStart(2, "0");
  const mm = d.getMinutes().toString().padStart(2, "0");
  const ss = d.getSeconds().toString().padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

export function radioFromLabel(
  msg: RadioMessage,
  options?: { viewerUserId?: number; selfLabel?: string },
): string {
  if (
    options?.viewerUserId != null &&
    msg.from.userId === options.viewerUserId &&
    options.selfLabel
  ) {
    return options.selfLabel;
  }
  return getUserName(msg.from);
}

export function isRadioSelfMessage(msg: RadioMessage, viewerUserId?: number): boolean {
  return viewerUserId != null && msg.from.userId === viewerUserId;
}

export function formatRadioChatLine(
  msg: RadioMessage,
  phraseLabel: string,
  options?: { viewerUserId?: number; selfLabel?: string },
): string {
  const fromLabel = radioFromLabel(msg, options);
  const body = `(${fromLabel}) ${contextLabel(msg)}: ${phraseLabel}`;
  return `${formatRadioMessageTime(msg.sentAt)} ${body}`;
}

export function radioMessagesNewestFirst(messages: RadioMessage[]): RadioMessage[] {
  return [...messages].sort((a, b) => b.sentAt - a.sentAt);
}

// radioMessageOpacity fades chat lines as they age: full strength for
// the first 5 minutes, then lightly at 5+, more at 10+, half at 30+.
export function radioMessageOpacity(sentAtMs: number, nowMs: number): number {
  const ageMs = nowMs - sentAtMs;
  if (ageMs <= 5 * 60_000) {
    return 1;
  }
  if (ageMs <= 10 * 60_000) {
    return 0.92;
  }
  if (ageMs <= 30 * 60_000) {
    return 0.72;
  }
  return 0.5;
}

export function radioContextFromMessage(msg: RadioMessage): RadioSendContext {
  if (msg.context.vehicle) {
    return { vehicleId: msg.context.vehicle.id };
  }
  if (msg.context.train) {
    return { trainId: msg.context.train.id };
  }
  return {};
}

export interface DriverRadioReplyTarget {
  to: RadioSendTarget;
  context: RadioSendContext;
  targetLabel: string;
  contextLabel: string;
}

// driverReplyTargetFromInbound builds a radio.send target for replying
// to a signalman message on throttle (interlocking + same context).
export function driverReplyTargetFromInbound(
  msg: RadioMessage,
): DriverRadioReplyTarget | null {
  const ilk = msg.fromInterlocking;
  if (ilk?.id == null) {
    return null;
  }
  const context = radioContextFromMessage(msg);
  if (context.vehicleId == null && context.trainId == null) {
    return null;
  }
  return {
    to: { interlockingId: ilk.id },
    context,
    targetLabel: ilk.name,
    contextLabel: contextLabel(msg),
  };
}

// signalmanReplyTargetFromInbound builds a radio.send target for replying
// to a driver message in the interlocking view (driver + same context).
export function signalmanReplyTargetFromInbound(
  msg: RadioMessage,
): DriverRadioReplyTarget | null {
  if (msg.to.interlockingId == null) {
    return null;
  }
  const context = radioContextFromMessage(msg);
  if (context.vehicleId == null && context.trainId == null) {
    return null;
  }
  return {
    to: { userId: msg.from.userId },
    context,
    targetLabel: getUserName(msg.from),
    contextLabel: contextLabel(msg),
  };
}

// isInboundRadioForDriver reports whether a radio.message push should
// notify the driver on throttle (signalman → driver, not own echo).
export function isInboundRadioForDriver(msg: RadioMessage, userId: number): boolean {
  if (msg.from.userId === userId) {
    return false;
  }
  return msg.to.userId === userId;
}

// isInboundRadioForInterlocking reports whether a radio.message push
// should notify the signalman occupying interlockingId (driver → box).
export function isInboundRadioForInterlocking(
  msg: RadioMessage,
  userId: number,
  interlockingId: number,
): boolean {
  if (msg.from.userId === userId) {
    return false;
  }
  return msg.to.interlockingId === interlockingId;
}
