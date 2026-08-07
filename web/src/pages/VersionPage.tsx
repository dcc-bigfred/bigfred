import { Container, Stack } from "@mui/material";

import VersionCard, { VersionPageHeader } from "../components/VersionCard";

export default function VersionPage() {
  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <VersionPageHeader />
        <VersionCard />
      </Stack>
    </Container>
  );
}
