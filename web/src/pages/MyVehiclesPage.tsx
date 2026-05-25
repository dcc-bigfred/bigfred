import { Box, Container, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import MyVehiclesCatalogue from "../components/MyVehiclesCatalogue";

export default function MyVehiclesPage() {
  const { t } = useTranslation(["vehicle"]);
  const me = useMe().data;

  if (!me) {
    return null;
  }

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Typography variant="h5" gutterBottom>
        {t("vehicle:list.title")}
      </Typography>
      <Box sx={{ mt: 2 }}>
        <MyVehiclesCatalogue layoutId={me.layoutId} />
      </Box>
    </Container>
  );
}
