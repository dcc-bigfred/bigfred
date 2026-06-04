import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";

export type FunctionSource = "template" | "vehicle";

export interface DccFunction {
  num: number;
  name: string;
  icon: string;
  position: number;
  source?: FunctionSource;
}

export interface VehicleTemplate {
  id: number;
  name: string;
  description: string;
  ownerId: number;
  version: number;
}

const iconsQueryKey = ["function-icons"] as const;
const templatesQueryKey = ["vehicle-templates"] as const;

export function vehicleFunctionsQueryKey(vehicleId: number) {
  return ["vehicles", vehicleId, "functions"] as const;
}

export function templateFunctionsQueryKey(templateId: number) {
  return ["vehicle-templates", templateId, "functions"] as const;
}

export function useFunctionIcons() {
  return useQuery({
    queryKey: iconsQueryKey,
    queryFn: () => apiFetch<{ icon: string }[]>("/api/v1/function-icons"),
    staleTime: 60 * 60 * 1000,
  });
}

export function useVehicleTemplates() {
  return useQuery({
    queryKey: templatesQueryKey,
    queryFn: () => apiFetch<VehicleTemplate[]>("/api/v1/vehicle-templates"),
    staleTime: 10 * 1000,
  });
}

export function useCreateVehicleTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; description?: string }) =>
      apiFetch<VehicleTemplate>("/api/v1/vehicle-templates", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: templatesQueryKey });
    },
  });
}

export function useVehicleFunctions(vehicleId: number) {
  return useQuery({
    queryKey: vehicleFunctionsQueryKey(vehicleId),
    queryFn: () =>
      apiFetch<DccFunction[]>(`/api/v1/vehicles/${vehicleId}/functions`),
    enabled: vehicleId > 0,
  });
}

export function useTemplateFunctions(templateId: number) {
  return useQuery({
    queryKey: templateFunctionsQueryKey(templateId),
    queryFn: () =>
      apiFetch<DccFunction[]>(
        `/api/v1/vehicle-templates/${templateId}/functions`,
      ),
    enabled: templateId > 0,
  });
}

export interface FunctionUpsertBody {
  name: string;
  icon: string;
  position: number;
}

export function useUpsertVehicleFunction(vehicleId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { num: number; body: FunctionUpsertBody }) =>
      apiFetch<DccFunction>(
        `/api/v1/vehicles/${vehicleId}/functions/${args.num}`,
        {
          method: "PUT",
          body: JSON.stringify(args.body),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehicleFunctionsQueryKey(vehicleId) });
    },
  });
}

export function useDeleteVehicleFunction(vehicleId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (num: number) =>
      apiFetch<void>(`/api/v1/vehicles/${vehicleId}/functions/${num}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehicleFunctionsQueryKey(vehicleId) });
    },
  });
}

export function useReorderVehicleFunctions(vehicleId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (positions: { num: number; position: number }[]) =>
      apiFetch<DccFunction[]>(
        `/api/v1/vehicles/${vehicleId}/functions/reorder`,
        {
          method: "POST",
          body: JSON.stringify({ positions }),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: vehicleFunctionsQueryKey(vehicleId) });
    },
  });
}

export function useUpsertTemplateFunction(templateId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { num: number; body: FunctionUpsertBody }) =>
      apiFetch<DccFunction>(
        `/api/v1/vehicle-templates/${templateId}/functions/${args.num}`,
        {
          method: "PUT",
          body: JSON.stringify(args.body),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: templateFunctionsQueryKey(templateId),
      });
    },
  });
}

export function useDeleteTemplateFunction(templateId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (num: number) =>
      apiFetch<void>(
        `/api/v1/vehicle-templates/${templateId}/functions/${num}`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: templateFunctionsQueryKey(templateId),
      });
    },
  });
}

export function useReorderTemplateFunctions(templateId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (positions: { num: number; position: number }[]) =>
      apiFetch<DccFunction[]>(
        `/api/v1/vehicle-templates/${templateId}/functions/reorder`,
        {
          method: "POST",
          body: JSON.stringify({ positions }),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: templateFunctionsQueryKey(templateId),
      });
    },
  });
}
