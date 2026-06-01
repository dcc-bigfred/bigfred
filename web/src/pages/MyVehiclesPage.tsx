import { Box, Container, Link, Typography } from "@mui/material";
import { Trans, useTranslation } from "react-i18next";
import { Link as RouterLink } from "react-router-dom";

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
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2, maxWidth: 720 }}>
        <Trans
          ns="vehicle"
          i18nKey="list.intro"
          components={{
            trainsLink: <Link component={RouterLink} to="/my/trains" />,
            addBtn: <Box component="span" sx={{ fontWeight: 600 }} />,
          }}
        />
      </Typography>
      <Box sx={{ mt: 2 }}>
        <MyVehiclesCatalogue layoutId={me.layoutId} />
      </Box>
    </Container>
  );
}
