import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";

// VehicleKind mirrors the closed catalogue defined in
// `pkgs/server/domain/vehicle.go`. The UI uses the value as a
// dropdown key and as the i18n suffix (`vehicle:kind.${kind}`).
export type VehicleKind =
  | "loco"
  | "emu"
  | "driving_wagon"
  | "trolley"
  | "wagon";

export const VEHICLE_KINDS: VehicleKind[] = [
  "loco",
  "emu",
  "driving_wagon",
  "trolley",
  "wagon",
];

export type DeadManSwitchOption =
  | "stop"
  | "stop_horn"
  | "stop_horn_emergency_lights";

export const DEADMAN_SWITCH_OPTIONS: DeadManSwitchOption[] = [
  "stop",
  "stop_horn",
  "stop_horn_emergency_lights",
];

export const DCC_FUNCTION_NUMBERS = Array.from({ length: 32 }, (_, i) => i);

export const DEFAULT_RP1_FUNCTION = 2;
export const DEFAULT_EMERGENCY_LIGHTS_FUNCTION = 0;
export const DEFAULT_DEADMAN_SWITCH_OPTION: DeadManSwitchOption = "stop";

// Vehicle is the JSON shape returned by `/api/v1/vehicles`. `dccAddress`
// is nullable because dummy vehicles (unpowered wagons, visual
// fillers) live in the catalogue without a DCC decoder.
export interface Vehicle {
  id: number;
  name: string;
  kind: VehicleKind;
  number: string;
  dccAddress: number | null;
  isDummy: boolean;
  ownerId: number;
  rp1Function: number;
  emergencyLightsFunction: number;
  deadManSwitchOption: DeadManSwitchOption;
}

// DCCAddressRange mirrors the row shape of
// `GET /api/v1/auth/me/dcc-pool`. The dialog uses it to render a
// "Your pool: 1..9999" hint.
export interface DCCAddressRange {
  from: number;
  to: number;
}

const vehiclesQueryKey = ["vehicles"] as const;
const dccPoolQueryKey = ["dcc-pool", "me"] as const;

// useMyVehicles returns the caller's own vehicle catalogue and re-
// fetches eagerly whenever the layout dashboard fires a
// `layout.vehiclesChanged` event so add/remove from the roster
// stays visually consistent with the catalogue.
export function useMyVehicles() {
  return useQuery({
    queryKey: vehiclesQueryKey,
    queryFn: () => apiFetch<Vehicle[]>("/api/v1/vehicles"),
    staleTime: 5 * 1000,
  });
}

// useMyDCCPool returns the caller's DCC pool — used by the vehicle
// add/edit dialog to validate the entered address client-side.
export function useMyDCCPool() {
  return useQuery({
    queryKey: dccPoolQueryKey,
    queryFn: () => apiFetch<DCCAddressRange[]>("/api/v1/auth/me/dcc-pool"),
    staleTime: 30 * 1000,
  });
}

export interface VehicleCreateBody {
  name: string;
  kind: VehicleKind;
  number: string;
  dccAddress: number | null;
  rp1Function: number;
  emergencyLightsFunction: number;
  deadManSwitchOption: DeadManSwitchOption;
}

export function useCreateVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: VehicleCreateBody) =>
      apiFetch<Vehicle>("/api/v1/vehicles", {
        method: "POST",
        body: JSON.stringify({
          name: body.name,
          kind: body.kind,
          number: body.number,
          dccAddress: body.dccAddress,
          rp1Function: body.rp1Function,
          emergencyLightsFunction: body.emergencyLightsFunction,
          deadManSwitchOption: body.deadManSwitchOption,
        }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehiclesQueryKey });
    },
  });
}

export interface VehicleUpdateBody {
  id: number;
  name?: string;
  kind?: VehicleKind;
  number?: string;
  // `dccAddress` carries the tri-state encoded by the backend:
  //   * undefined           — leave the column alone;
  //   * null                — mark as dummy;
  //   * a number            — set / change the address.
  dccAddress?: number | null;
  rp1Function?: number;
  emergencyLightsFunction?: number;
  deadManSwitchOption?: DeadManSwitchOption;
}

export function useUpdateVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: VehicleUpdateBody) => {
      const payload: Record<string, unknown> = {};
      if (body.name !== undefined) payload.name = body.name;
      if (body.kind !== undefined) payload.kind = body.kind;
      if (body.number !== undefined) payload.number = body.number;
      if (body.dccAddress !== undefined) {
        payload.dccAddress = body.dccAddress;
        payload.dccAddressSet = true;
      }
      if (body.rp1Function !== undefined) {
        payload.rp1Function = body.rp1Function;
      }
      if (body.emergencyLightsFunction !== undefined) {
        payload.emergencyLightsFunction = body.emergencyLightsFunction;
      }
      if (body.deadManSwitchOption !== undefined) {
        payload.deadManSwitchOption = body.deadManSwitchOption;
      }
      return apiFetch<Vehicle>(`/api/v1/vehicles/${body.id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehiclesQueryKey });
    },
  });
}

export function useDeleteVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/vehicles/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehiclesQueryKey });
    },
  });
}

// -----------------------------------------------------------------
// Trains
// -----------------------------------------------------------------

export interface TrainMember {
  id: number;
  vehicleId: number;
  position: number;
  reversed: boolean;
  speedMultiplier: number;
}

export interface Train {
  id: number;
  name: string;
  ownerId: number;
  members: TrainMember[];
}

const trainsQueryKey = ["trains"] as const;

export function useMyTrains() {
  return useQuery({
    queryKey: trainsQueryKey,
    queryFn: () => apiFetch<Train[]>("/api/v1/trains"),
    staleTime: 5 * 1000,
  });
}

export interface TrainMemberInput {
  vehicleId: number;
  reversed: boolean;
  speedMultiplier?: number;
}

export interface TrainCreateBody {
  name: string;
  members: TrainMemberInput[];
}

export function useCreateTrain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: TrainCreateBody) =>
      apiFetch<Train>("/api/v1/trains", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: trainsQueryKey });
    },
  });
}

export interface TrainUpdateBody {
  id: number;
  name?: string;
  members?: TrainMemberInput[];
}

export function useUpdateTrain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: TrainUpdateBody) => {
      const payload: Record<string, unknown> = {};
      if (body.name !== undefined) payload.name = body.name;
      if (body.members !== undefined) {
        payload.members = body.members;
        payload.membersSet = true;
      }
      return apiFetch<Train>(`/api/v1/trains/${body.id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: trainsQueryKey });
    },
  });
}

export function useDeleteTrain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/trains/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: trainsQueryKey });
    },
  });
}

// -----------------------------------------------------------------
// Layout roster (dashboard data source, live over WS)
// -----------------------------------------------------------------

export interface RosterVehicle extends Vehicle {
  ownerLogin: string;
  addedAt: string;
  canDrive?: boolean;
}

export interface RosterTrain {
  id: number;
  name: string;
  ownerId: number;
  ownerLogin: string;
  addedAt: string;
  canDrive?: boolean;
  members: TrainMember[];
}

export function layoutVehiclesQueryKey(layoutId: number) {
  return ["layouts", layoutId, "vehicles"] as const;
}

export function layoutTrainsQueryKey(layoutId: number) {
  return ["layouts", layoutId, "trains"] as const;
}

// useLayoutVehicles loads the layout vehicle roster and re-fetches
// on every `layout.vehiclesChanged` WS event so the dashboard table
// stays live without polling — mirroring how `useDashboardInterlockings`
// merges `interlocking.occupantChanged`.
export function useLayoutVehicles(layoutId: number | null) {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  const query = useQuery({
    queryKey: layoutVehiclesQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<RosterVehicle[]>(`/api/v1/layouts/${layoutId}/vehicles`),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 2 * 1000,
  });

  useEffect(() => {
    if (layoutId == null || layoutId <= 0) return;
    return subscribe("layout.vehiclesChanged", (payload) => {
      const data = payload as { layoutId?: number };
      if (data.layoutId !== layoutId) return;
      void qc.invalidateQueries({ queryKey: layoutVehiclesQueryKey(layoutId) });
    });
  }, [layoutId, subscribe, qc]);

  return query;
}

// useLayoutTrains mirrors useLayoutVehicles for the train roster.
export function useLayoutTrains(layoutId: number | null) {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  const query = useQuery({
    queryKey: layoutTrainsQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<RosterTrain[]>(`/api/v1/layouts/${layoutId}/trains`),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 2 * 1000,
  });

  useEffect(() => {
    if (layoutId == null || layoutId <= 0) return;
    return subscribe("layout.trainsChanged", (payload) => {
      const data = payload as { layoutId?: number };
      if (data.layoutId !== layoutId) return;
      void qc.invalidateQueries({ queryKey: layoutTrainsQueryKey(layoutId) });
    });
  }, [layoutId, subscribe, qc]);

  return query;
}

export function useAddVehicleToRoster() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; vehicleId: number }) =>
      apiFetch<RosterVehicle>(
        `/api/v1/layouts/${args.layoutId}/vehicles`,
        {
          method: "POST",
          body: JSON.stringify({ vehicleId: args.vehicleId }),
        },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutVehiclesQueryKey(args.layoutId),
      });
    },
  });
}

export function useRemoveVehicleFromRoster() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; vehicleId: number }) =>
      apiFetch<void>(
        `/api/v1/layouts/${args.layoutId}/vehicles/${args.vehicleId}`,
        { method: "DELETE" },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutVehiclesQueryKey(args.layoutId),
      });
    },
  });
}

export function useAddTrainToRoster() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; trainId: number }) =>
      apiFetch<RosterTrain>(
        `/api/v1/layouts/${args.layoutId}/trains`,
        {
          method: "POST",
          body: JSON.stringify({ trainId: args.trainId }),
        },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutTrainsQueryKey(args.layoutId),
      });
    },
  });
}

export function useRemoveTrainFromRoster() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; trainId: number }) =>
      apiFetch<void>(
        `/api/v1/layouts/${args.layoutId}/trains/${args.trainId}`,
        { method: "DELETE" },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutTrainsQueryKey(args.layoutId),
      });
    },
  });
}


export function usePatchTrainMemberMultiplier() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      trainId,
      memberId,
      speedMultiplier,
    }: {
      trainId: number;
      memberId: number;
      speedMultiplier: number;
    }) =>
      apiFetch<TrainMember>(
        `/api/v1/trains/${trainId}/members/${memberId}`,
        {
          method: "PATCH",
          body: JSON.stringify({ speedMultiplier }),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: trainsQueryKey });
      void qc.invalidateQueries({ queryKey: ["layouts"] });
    },
  });
}
