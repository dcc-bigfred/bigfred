import { useState, type ReactNode } from "react";
import {
  Button,
  Divider,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from "@mui/material";
import ArrowDropDownIcon from "@mui/icons-material/ArrowDropDown";

// TopBarMenuItem is the shape every entry in a dropdown takes. Keeping
// the items as a data array (rather than children) lets the parent
// describe a menu declaratively, makes role-based filtering trivial
// (just .filter() before passing in), and keeps the rendering /
// accessibility wiring centralised here.
//
// Discriminating between "real" items and a divider via the `divider`
// flag is more ergonomic than mixing two component types in JSX
// children, especially when items are computed in a loop.
export type TopBarMenuItem =
  | {
      /** Unique within the menu — used as React key + aria id. */
      id: string;
      /** Already-translated text the user sees. */
      label: ReactNode;
      /** Optional leading icon (any MUI icon). */
      icon?: ReactNode;
      /** Click handler; ignored when `disabled` is true. */
      onClick?: () => void;
      /** Greyed out + non-interactive when true. */
      disabled?: boolean;
      /**
       * Optional tooltip shown on hover even when `disabled`. Used to
       * explain WHY a stubbed-out item isn't usable yet (e.g.
       * "Coming in M3").
       */
      tooltip?: string;
      divider?: never;
    }
  | {
      /** Renders a horizontal MUI Divider between items. */
      divider: true;
      id: string;
      label?: never;
      icon?: never;
      onClick?: never;
      disabled?: never;
      tooltip?: never;
    };

interface TopBarMenuProps {
  /** Visible button label (already translated). */
  label: string;
  /** Optional secondary label rendered in a lower-emphasis tone next
   *  to the main one — currently used by the Account menu to show the
   *  current login alongside the menu name. */
  caption?: string;
  /** Accessible label for the button (defaults to `label`). */
  ariaLabel?: string;
  /** Menu contents. Falsy items are filtered out, so the caller can
   *  inline conditional entries with `&&`. */
  items: (TopBarMenuItem | false | null | undefined)[];
}

// TopBarMenu wraps an MUI Button + Menu in a single component that
// behaves like a classic desktop application menu (think File / Edit
// / View). Multiple TopBarMenu instances side-by-side in the AppBar
// give the user a familiar horizontal menu strip.
//
// Implementation notes:
//   - Anchor lives in component state so each menu opens/closes
//     independently — clicking "Administration" does not close
//     "Account" if it happens to also be open.
//   - We render disabled items with a wrapping <span> around the
//     MenuItem so the Tooltip still fires on hover (MUI Tooltip
//     ignores disabled children by default).
//   - `inherit` colour on the Button + endIcon matches the surrounding
//     AppBar's contrast palette.
export default function TopBarMenu({
  label,
  caption,
  ariaLabel,
  items,
}: TopBarMenuProps) {
  const [anchor, setAnchor] = useState<null | HTMLElement>(null);
  const menuId = `topbar-menu-${label.toLowerCase().replace(/\s+/g, "-")}`;

  const open = Boolean(anchor);
  const close = () => setAnchor(null);

  // Drop falsy entries (lets the caller write `isAdmin && {...}`).
  const visible = items.filter(
    (it): it is TopBarMenuItem => it !== false && it != null,
  );

  return (
    <>
      <Button
        color="inherit"
        size="medium"
        aria-label={ariaLabel ?? label}
        aria-haspopup="menu"
        aria-controls={open ? menuId : undefined}
        aria-expanded={open ? "true" : undefined}
        endIcon={<ArrowDropDownIcon />}
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{
          // Match the AppBar's typography weight (medium-emphasis
          // links don't shout for attention).
          textTransform: "none",
          fontWeight: 500,
          gap: 0.5,
        }}
      >
        {label}
        {caption && (
          <Typography
            component="span"
            variant="body2"
            sx={{ opacity: 0.75, ml: 0.5, fontWeight: 400 }}
          >
            {caption}
          </Typography>
        )}
      </Button>

      <Menu
        id={menuId}
        anchorEl={anchor}
        open={open}
        onClose={close}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
        slotProps={{ list: { dense: false } }}
      >
        {visible.map((item) => {
          if (item.divider) {
            return <Divider key={item.id} component="li" />;
          }

          const handleClick = () => {
            close();
            item.onClick?.();
          };

          const menuItem = (
            <MenuItem
              key={item.id}
              onClick={item.onClick ? handleClick : undefined}
              disabled={item.disabled}
              sx={{ minWidth: 200 }}
            >
              {item.icon !== undefined && (
                <ListItemIcon>{item.icon}</ListItemIcon>
              )}
              <ListItemText primary={item.label} />
            </MenuItem>
          );

          // MUI Tooltip drops pointer events on disabled descendants,
          // so disabled items need a wrapping element for the
          // hover-target. The <span> keeps focus order intact.
          if (item.tooltip) {
            return (
              <Tooltip
                key={item.id}
                title={item.tooltip}
                placement="left"
                disableInteractive
              >
                <span>{menuItem}</span>
              </Tooltip>
            );
          }

          return menuItem;
        })}
      </Menu>
    </>
  );
}
