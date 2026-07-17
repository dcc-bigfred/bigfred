import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";

export type CommandStationKind =
  | "z21"
  | "loconet_serial"
  | "loconet_tcp";

export const COMMAND_STATION_KINDS: CommandStationKind[] = [
  "z21",
  "loconet_serial",
  "loconet_tcp",
];

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
