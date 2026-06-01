import {
  Box,
  Divider,
  Drawer,
  IconButton,
  List,
  ListSubheader,
  Toolbar,
  Typography,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";

import { type TopBarMenuItem, TopBarMenuListContent } from "./TopBarMenu";

export interface MobileNavSection {
  id: string;
  label: string;
  caption?: string;
  items: TopBarMenuItem[];
}

interface MobileNavDrawerProps {
  open: boolean;
  onClose: () => void;
  title: string;
  closeLabel: string;
  sections: MobileNavSection[];
  /** Optional identity line shown under the drawer header. */
  identityLine?: string;
}

// MobileNavDrawer is the slide-out navigation used below the `md`
// breakpoint. It mirrors the three TopBarMenu dropdowns (My,
// Administration, Account) as stacked sections so nothing is lost on
// narrow screens.
export default function MobileNavDrawer({
  open,
  onClose,
  title,
  closeLabel,
  sections,
  identityLine,
}: MobileNavDrawerProps) {
  return (
    <Drawer
      anchor="left"
      open={open}
      onClose={onClose}
      ModalProps={{ keepMounted: true }}
      slotProps={{
        paper: { sx: { width: 280, maxWidth: "85vw" } },
      }}
    >
      <Toolbar
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          gap: 1,
        }}
      >
        <Typography variant="h6" component="span" noWrap sx={{ flex: 1 }}>
          {title}
        </Typography>
        <IconButton
          edge="end"
          aria-label={closeLabel}
          onClick={onClose}
          sx={{ ml: "auto" }}
        >
          <CloseIcon />
        </IconButton>
      </Toolbar>

      {identityLine && (
        <Box sx={{ px: 2, pb: 1 }}>
          <Typography variant="body2" color="text.secondary" noWrap>
            {identityLine}
          </Typography>
        </Box>
      )}

      <Divider />

      <Box component="nav" sx={{ overflowY: "auto", flexGrow: 1 }}>
        {sections.map((section, index) => (
          <List
            key={section.id}
            dense={false}
            subheader={
              <ListSubheader component="div" disableSticky>
                {section.label}
                {section.caption && (
                  <Typography
                    component="span"
                    variant="caption"
                    display="block"
                    color="text.secondary"
                    sx={{ fontWeight: 400, lineHeight: 1.4 }}
                  >
                    {section.caption}
                  </Typography>
                )}
              </ListSubheader>
            }
          >
            <TopBarMenuListContent
              items={section.items}
              onItemActivate={onClose}
            />
            {index < sections.length - 1 && <Divider />}
          </List>
        ))}
      </Box>
    </Drawer>
  );
}
