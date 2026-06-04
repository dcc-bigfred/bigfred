import { Container } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import {
  useDeleteTemplateFunction,
  useReorderTemplateFunctions,
  useTemplateFunctions,
  useUpsertTemplateFunction,
  useVehicleTemplates,
} from "../api/functions";
import FunctionListEditor from "../components/functions/FunctionListEditor";

export default function TemplateFunctionsPage() {
  const { templateId: templateIdParam } = useParams();
  const templateId = Number(templateIdParam);
  const navigate = useNavigate();
  const { t } = useTranslation(["function"]);
  const templates = useVehicleTemplates();
  const functions = useTemplateFunctions(templateId);
  const upsert = useUpsertTemplateFunction(templateId);
  const remove = useDeleteTemplateFunction(templateId);
  const reorder = useReorderTemplateFunctions(templateId);

  const template = templates.data?.find((x) => x.id === templateId);

  if (!templateId || Number.isNaN(templateId)) {
    navigate("/vehicle-templates");
    return null;
  }

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <FunctionListEditor
        mode="template"
        title={t("function:editor.templateTitle", {
          name: template?.name ?? "…",
        })}
        onBack={() => navigate("/vehicle-templates")}
        functions={functions.data}
        isLoading={functions.isLoading}
        mutations={{
          upsert,
          remove,
          reorder,
        }}
      />
    </Container>
  );
}
