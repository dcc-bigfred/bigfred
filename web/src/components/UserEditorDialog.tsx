import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Stack,
  TextField,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import type { Role } from "../api/auth";
import {
  USER_MANAGEABLE_ROLES,
  useCreateUser,
  useUpdateUser,
  type User,
} from "../api/users";
import UserDccPoolFields, {
  dccPoolFromApi,
  emptyDccPoolRange,
  isDccPoolInputValid,
  parseDccPoolRanges,
  type DccPoolRangeInput,
} from "./UserDccPoolFields";
import { getUserName } from "../utils/getUserName";

export type UserEditorDialogMode =
  | { kind: "create" }
  | { kind: "edit"; target: User };

type Props = {
  mode: UserEditorDialogMode | null;
  onClose: () => void;
};

/** Create / edit user dialog shared by Users admin and DCC pools view. */
export default function UserEditorDialog({ mode, onClose }: Props) {
  const { t } = useTranslation(["user", "common", "errors", "role"]);
  const create = useCreateUser();
  const update = useUpdateUser();

  const [loginInput, setLoginInput] = useState("");
  const [organizationInput, setOrganizationInput] = useState("");
  const [pinInput, setPinInput] = useState("");
  const [roleInput, setRoleInput] = useState<Role>("driver");
  const [dccPoolInput, setDccPoolInput] = useState<DccPoolRangeInput[]>([
    emptyDccPoolRange(),
  ]);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (!mode) return;
    setActionError(null);
    create.reset();
    update.reset();
    if (mode.kind === "create") {
      setLoginInput("");
      setOrganizationInput("");
      setPinInput("");
      setRoleInput("driver");
      setDccPoolInput([emptyDccPoolRange()]);
      return;
    }
    setLoginInput(mode.target.login);
    setOrganizationInput(mode.target.organization ?? "");
    setPinInput("");
    setRoleInput(mode.target.role);
    setDccPoolInput(dccPoolFromApi(mode.target.dccPool ?? []));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reset only when dialog target/mode changes
  }, [mode]);

  const handleClose = () => {
    setActionError(null);
    create.reset();
    update.reset();
    onClose();
  };

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const submitting = create.isPending || update.isPending;
  const trimmedLogin = loginInput.trim();
  const loginValid = /^[A-Za-z0-9._-]{1,32}$/.test(trimmedLogin);
  const pinValid = /^[0-9]{4,12}$/.test(pinInput);
  const dccPoolValid = isDccPoolInputValid(dccPoolInput);
  const createValid = loginValid && pinValid && dccPoolValid;
  const editValid =
    loginValid && (pinInput === "" || pinValid) && dccPoolValid;

  const renderRole = (role: Role) =>
    t(`role:${role}` as const, { defaultValue: role });

  const submit = async () => {
    if (!mode) return;
    try {
      if (mode.kind === "create") {
        const dccPool = parseDccPoolRanges(dccPoolInput);
        if (!dccPool) return;
        await create.mutateAsync({
          login: loginInput.trim(),
          organization: organizationInput.trim(),
          pin: pinInput,
          role: roleInput,
          dccPool,
        });
      } else {
        const dccPool = parseDccPoolRanges(dccPoolInput);
        if (!dccPool) return;
        await update.mutateAsync({
          id: mode.target.id,
          login: loginInput.trim(),
          organization: organizationInput.trim(),
          role: roleInput,
          pin: pinInput || undefined,
          dccPool,
        });
      }
      handleClose();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  return (
    <Dialog
      open={mode != null}
      onClose={handleClose}
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle>
        {mode?.kind === "edit"
          ? t("user:admin.dialogs.edit.title", {
              login: getUserName(mode.target),
            })
          : t("user:admin.dialogs.create.title")}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField
            label={t("user:admin.dialogs.fields.login")}
            value={loginInput}
            onChange={(e) => setLoginInput(e.target.value)}
            helperText={t("user:admin.dialogs.fields.loginHelp")}
            autoFocus
            fullWidth
            required
          />
          <TextField
            label={t("user:admin.dialogs.fields.organization")}
            value={organizationInput}
            onChange={(e) => setOrganizationInput(e.target.value)}
            helperText={t("user:admin.dialogs.fields.organizationHelp")}
            fullWidth
            inputProps={{ maxLength: 128 }}
          />
          <TextField
            select
            label={t("user:admin.dialogs.fields.role")}
            value={roleInput}
            onChange={(e) => setRoleInput(e.target.value as Role)}
            fullWidth
          >
            {USER_MANAGEABLE_ROLES.map((role) => (
              <MenuItem key={role} value={role}>
                {renderRole(role)}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label={t("user:admin.dialogs.fields.pin")}
            type="password"
            value={pinInput}
            onChange={(e) => setPinInput(e.target.value)}
            helperText={
              mode?.kind === "edit"
                ? t("user:admin.dialogs.fields.pinEditHelp")
                : t("user:admin.dialogs.fields.pinCreateHelp")
            }
            placeholder={
              mode?.kind === "edit"
                ? t("user:admin.dialogs.fields.pinPlaceholder")
                : undefined
            }
            required={mode?.kind === "create"}
            fullWidth
            inputProps={{ inputMode: "numeric", pattern: "[0-9]*" }}
          />
          <UserDccPoolFields
            value={dccPoolInput}
            onChange={setDccPoolInput}
            disabled={submitting}
          />
          {actionError && <Alert severity="error">{actionError}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={submitting}>
          {t("common:actions.cancel")}
        </Button>
        <Button
          variant="contained"
          onClick={() => void submit()}
          disabled={
            submitting ||
            (mode?.kind === "create" && !createValid) ||
            (mode?.kind === "edit" && !editValid)
          }
        >
          {mode?.kind === "edit"
            ? t("user:admin.dialogs.edit.submit")
            : t("user:admin.dialogs.create.submit")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
