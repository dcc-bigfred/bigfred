import { Container, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

import GrantedLeasesPanel from "../../components/leases/GrantedLeasesPanel";

export default function AdminRentalsPage() {
  const { t } = useTranslation(["rentals"]);

  return (
    <Container maxWidth="md" sx={{ py: 3 }}>
      <Typography variant="h5" gutterBottom>
        {t("admin.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2, maxWidth: 720 }}>
        {t("admin.intro")}
      </Typography>
      <GrantedLeasesPanel allowUnresolvedTarget showOwner />
    </Container>
  );
}
