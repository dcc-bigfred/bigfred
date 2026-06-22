import { useMemo } from "react";
import { Container } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import {
  useDeleteVehicleFunction,
  useFunctionCatalogue,
  useReorderVehicleFunctions,
  useReplaceVehicleFunctionsFromSource,
  useUpsertVehicleFunction,
  useVehicleFunctions,
  useVehicleTemplates,
  type FunctionCopySource,
} from "../api/functions";
import { useMyVehicles } from "../api/vehicles";
import FunctionListEditor from "../components/functions/FunctionListEditor";

export default function VehicleFunctionsPage() {
  const { vehicleId: vehicleIdParam } = useParams();
  const vehicleId = vehicleIdParam ?? "";
  const navigate = useNavigate();
  const { t } = useTranslation(["vehicle", "function"]);
  const vehicles = useMyVehicles();
  const functions = useVehicleFunctions(vehicleId);
  const templates = useVehicleTemplates();
  const catalogue = useFunctionCatalogue(true);
  const upsert = useUpsertVehicleFunction(vehicleId);
  const remove = useDeleteVehicleFunction(vehicleId);
  const reorder = useReorderVehicleFunctions(vehicleId);
  const replaceFromSource = useReplaceVehicleFunctionsFromSource(vehicleId);

  const vehicle = vehicles.data?.find((v) => v.id === vehicleId);

  const copySources = useMemo((): FunctionCopySource[] => {
    const templateRows: FunctionCopySource[] = (templates.data ?? []).map(
      (row) => ({
        kind: "template",
        id: row.id,
        name: row.name,
        ownerLogin: row.ownerLogin,
        ownerOrganization: row.ownerOrganization,
      }),
    );
    const locomotiveRows: FunctionCopySource[] = (catalogue.data ?? [])
      .filter((entry) => entry.vehicleId !== vehicleId)
      .map((entry) => ({
        kind: "locomotive",
        id: entry.vehicleId,
        name: entry.vehicleName,
        ownerLogin: entry.ownerLogin,
        ownerOrganization: entry.ownerOrganization,
      }));
    const byName = (a: FunctionCopySource, b: FunctionCopySource) =>
      a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
    return [...templateRows.sort(byName), ...locomotiveRows.sort(byName)];
  }, [templates.data, catalogue.data, vehicleId]);

  const inheritedBanner = useMemo(
    () => (functions.data ?? []).some((f) => f.source === "template"),
    [functions.data],
  );

  if (!vehicleId) {
    navigate("/my/vehicles");
    return null;
  }

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <FunctionListEditor
        mode="vehicle"
        title={t("function:editor.vehicleTitle", {
          name: vehicle?.name ?? "…",
        })}
        subtitle={
          vehicle?.dccAddress != null
            ? t("function:editor.dccSubtitle", { addr: vehicle.dccAddress })
            : undefined
        }
        onBack={() => navigate("/my/vehicles")}
        functions={functions.data}
        isLoading={functions.isLoading}
        inheritedBanner={inheritedBanner}
        mutations={{
          upsert,
          remove,
          reorder,
        }}
        copyFunctionsFrom={{
          sources: copySources,
          isLoadingSources: templates.isLoading || catalogue.isLoading,
          replace: replaceFromSource,
        }}
      />
    </Container>
  );
}
