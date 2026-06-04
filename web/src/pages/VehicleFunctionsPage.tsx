import { useMemo } from "react";
import { Container } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import {
  useDeleteVehicleFunction,
  useReorderVehicleFunctions,
  useUpsertVehicleFunction,
  useVehicleFunctions,
} from "../api/functions";
import { useMyVehicles } from "../api/vehicles";
import FunctionListEditor from "../components/functions/FunctionListEditor";

export default function VehicleFunctionsPage() {
  const { vehicleId: vehicleIdParam } = useParams();
  const vehicleId = Number(vehicleIdParam);
  const navigate = useNavigate();
  const { t } = useTranslation(["vehicle", "function"]);
  const vehicles = useMyVehicles();
  const functions = useVehicleFunctions(vehicleId);
  const upsert = useUpsertVehicleFunction(vehicleId);
  const remove = useDeleteVehicleFunction(vehicleId);
  const reorder = useReorderVehicleFunctions(vehicleId);

  const vehicle = vehicles.data?.find((v) => v.id === vehicleId);

  const inheritedBanner = useMemo(
    () => (functions.data ?? []).some((f) => f.source === "template"),
    [functions.data],
  );

  if (!vehicleId || Number.isNaN(vehicleId)) {
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
      />
    </Container>
  );
}
