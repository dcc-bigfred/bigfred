import { Box, Container, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import MyTrainsCatalogue from "../components/MyTrainsCatalogue";

export default function MyTrainsPage() {
  const { t } = useTranslation(["vehicle"]);
  const me = useMe().data;

  if (!me) {
    return null;
  }

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Typography variant="h5" gutterBottom>
        {t("vehicle:trainList.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2, maxWidth: 720 }}>
        {t("vehicle:trainList.intro")}
      </Typography>
      <Box sx={{ mt: 2 }}>
        <MyTrainsCatalogue layoutId={me.layoutId} />
      </Box>
    </Container>
  );
}
