import { Box, Container, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import AvailableVehiclesCatalogue from "../components/AvailableVehiclesCatalogue";

export default function AvailableVehiclesPage() {
  const { t } = useTranslation(["vehicle"]);
  const me = useMe().data;

  if (!me) {
    return null;
  }

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Typography variant="h5" gutterBottom>
        {t("vehicle:catalogue.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2, maxWidth: 720 }}>
        {t("vehicle:catalogue.intro")}
      </Typography>
      <Box sx={{ mt: 2 }}>
        <AvailableVehiclesCatalogue layoutId={me.layoutId} />
      </Box>
    </Container>
  );
}
