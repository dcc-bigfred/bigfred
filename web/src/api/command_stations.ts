import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";
import { capabilities } from "../capabilities";

export type CommandStationKind =
  | "z21"
  | "loconet_serial"
  | "loconet_tcp";

const ALL_COMMAND_STATION_KINDS: CommandStationKind[] = [
  "z21",
  "loconet_serial",
  "loconet_tcp",
];

export const COMMAND_STATION_KINDS: CommandStationKind[] =
  capabilities.loconetSerial
    ? ALL_COMMAND_STATION_KINDS
    : ALL_COMMAND_STATION_KINDS.filter((k) => k !== "loconet_serial");

export const COMMAND_STATION_SPEED_STEPS = [14, 28, 128] as const;

export const DEFAULT_COMMAND_STATION_HEARTBEAT_SECS = 2;
export const DEFAULT_COMMAND_STATION_DEADMAN_SECS = 6;
export const DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS = 0;
export const DEFAULT_COMMAND_STATION_SPEED_STEPS = 128;
export const DEFAULT_COMMAND_STATION_MAX_LOCONET_SLOTS = 80;
export const DEFAULT_COMMAND_STATION_IDLE_TIMEOUT_SECS = 60;
export const DEFAULT_LAYOUT_MAX_VEHICLES_PER_USER = 8;

export interface CommandStation {
  id: number;
  name: string;
  kind: CommandStationKind;
  connectionUri: string;
  speedSteps: number;
  heartbeatSecs: number;
  deadmanSecs: number;
  pollIntervalMs: number;
  z21ServerEnabled: boolean;
  z21IpStickiness: boolean;
  withrottleServerEnabled: boolean;
  maxLoconetSlots?: number;
  idleTimeoutSecs?: number;
  bootStopEnabled: boolean;
  singleVehicleControl: boolean;
  allocatePhysicalSlots?: boolean;
}

const commandStationsCatalogueQueryKey = [
  "command-stations",
  "catalogue",
] as const;

export function useCommandStationsCatalogue() {
  return useQuery({
    queryKey: commandStationsCatalogueQueryKey,
    queryFn: () =>
      apiFetch<CommandStation[]>("/api/v1/command-stations/catalogue"),
    staleTime: 5 * 1000,
  });
}

export function useCreateCommandStation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      kind: CommandStationKind;
      connectionUri: string;
      speedSteps: number;
      heartbeatSecs: number;
      deadmanSecs: number;
      pollIntervalMs: number;
      z21ServerEnabled?: boolean;
      z21IpStickiness?: boolean;
      withrottleServerEnabled?: boolean;
      maxLoconetSlots?: number;
      idleTimeoutSecs?: number;
      bootStopEnabled?: boolean;
      singleVehicleControl?: boolean;
      allocatePhysicalSlots?: boolean;
    }) =>
      apiFetch<CommandStation>("/api/v1/command-stations", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: commandStationsCatalogueQueryKey });
    },
  });
}

export function useUpdateCommandStation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      id: number;
      name: string;
      kind: CommandStationKind;
      connectionUri: string;
      speedSteps: number;
      heartbeatSecs: number;
      deadmanSecs: number;
      pollIntervalMs: number;
      z21ServerEnabled?: boolean;
      z21IpStickiness?: boolean;
      withrottleServerEnabled?: boolean;
      maxLoconetSlots?: number;
      idleTimeoutSecs?: number;
      bootStopEnabled?: boolean;
      singleVehicleControl?: boolean;
      allocatePhysicalSlots?: boolean;
    }) =>
      apiFetch<CommandStation>(`/api/v1/command-stations/${args.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: args.name,
          kind: args.kind,
          connectionUri: args.connectionUri,
          speedSteps: args.speedSteps,
          heartbeatSecs: args.heartbeatSecs,
          deadmanSecs: args.deadmanSecs,
          pollIntervalMs: args.pollIntervalMs,
          z21ServerEnabled: args.z21ServerEnabled,
          z21IpStickiness: args.z21IpStickiness,
          withrottleServerEnabled: args.withrottleServerEnabled,
          maxLoconetSlots: args.maxLoconetSlots,
          idleTimeoutSecs: args.idleTimeoutSecs,
          bootStopEnabled: args.bootStopEnabled,
          singleVehicleControl: args.singleVehicleControl,
          allocatePhysicalSlots: args.allocatePhysicalSlots,
        }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: commandStationsCatalogueQueryKey });
    },
  });
}

export function useDeleteCommandStation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/command-stations/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: commandStationsCatalogueQueryKey });
    },
  });
}

export function layoutCommandStationsQueryKey(layoutId: number) {
  return ["layouts", layoutId, "command-stations"] as const;
}

export function useLayoutCommandStations(layoutId: number | null) {
  return useQuery({
    queryKey: layoutCommandStationsQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<CommandStation[]>(
        `/api/v1/layouts/${layoutId}/command-stations`,
      ),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 5 * 1000,
  });
}

export function useSetLayoutCommandStations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; commandStationIds: number[] }) =>
      apiFetch<CommandStation[]>(
        `/api/v1/layouts/${args.layoutId}/command-stations`,
        {
          method: "PUT",
          body: JSON.stringify({ commandStationIds: args.commandStationIds }),
        },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutCommandStationsQueryKey(args.layoutId),
      });
    },
  });
}

export interface DetectedConnection {
  name: string;
  uri: string;
}

export function kindFromConnectionUri(uri: string): CommandStationKind {
  const lower = uri.toLowerCase();
  if (lower.startsWith("serial://")) return "loconet_serial";
  if (lower.startsWith("udp://")) return "z21";
  return "loconet_tcp";
}

export function isSerialAutodetectUri(uri: string): boolean {
  return /^serial:\/\/autodetect(?::|$)/i.test(uri);
}

export function commandStationScanWsUrl(): string {
  const path = "/api/v1/command-stations/scan/ws";
  const base = (import.meta.env.VITE_API_BASE ?? "") as string;
  if (base) {
    const url = new URL(base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.pathname = path;
    url.search = "";
    return url.toString();
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${path}`;
}

export type ScanWsFrame =
  | { type: "connection"; name: string; uri: string }
  | { type: "error"; detail?: string }
  | { type: "done" };

export interface DccBusProgramStatus {
  layoutId: number;
  layoutName: string;
  name: string;
  status: string;
  pid?: number;
  running: boolean;
}

export interface DccBusSupervisordStatus {
  programs: DccBusProgramStatus[];
}

export type DccBusSupervisordAction = "start" | "stop" | "restart";

function dccBusSupervisordStatusQueryKey(csId: number) {
  return ["admin", "dcc-bus", csId, "supervisord"] as const;
}

export function useDccBusSupervisordStatus(csId: number | null) {
  return useQuery({
    queryKey: dccBusSupervisordStatusQueryKey(csId ?? 0),
    queryFn: () =>
      apiFetch<DccBusSupervisordStatus>(
        `/api/v1/admin/dcc-bus/${csId}/supervisord`,
      ),
    enabled: csId != null && csId > 0,
    staleTime: 2 * 1000,
    refetchInterval: 5 * 1000,
  });
}

export function useDccBusSupervisordAction(csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { action: DccBusSupervisordAction; layoutId: number }) =>
      apiFetch<void>(
        `/api/v1/admin/dcc-bus/${csId}/supervisord/${vars.action}?layoutId=${vars.layoutId}`,
        { method: "POST" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: dccBusSupervisordStatusQueryKey(csId),
      });
    },
  });
}
