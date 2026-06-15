import { useCallback, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";

export const RADIO_PHRASES = [
  "STOPPED_AT_SIGNAL_READY_TO_ENTER",
  "ENTRY_PERMITTED",
  "CANCEL_ROUTE",
  "ROUTE_SET",
  "ACK",
  "STOP_IMMEDIATELY",
  "READY_TO_DEPART",
  "DEPARTURE_CLEARED",
] as const;

export type RadioPhrase = (typeof RADIO_PHRASES)[number];

export interface RadioUser {
  userId: number;
  login: string;
}

export interface RadioTarget {
  userId?: number;
  interlockingId?: number;
}

export interface RadioContextEntity {
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
  fromInterlocking?: RadioContextEntity;
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
  vehicleId?: number;
  trainId?: number;
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

export function radioSenderLabel(msg: RadioMessage): string {
  if (msg.fromInterlocking?.name) {
    return msg.fromInterlocking.name;
  }
  return msg.from.login;
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

export function formatRadioChatLine(msg: RadioMessage, phraseLabel: string): string {
  const body = `(${msg.from.login}) ${contextLabel(msg)}: ${phraseLabel}`;
  return `${formatRadioMessageTime(msg.sentAt)} ${body}`;
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
