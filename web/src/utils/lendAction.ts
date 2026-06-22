import type { TFunction } from "i18next";

import { lendableTargetKey } from "../api/leases";

export function showLendButton(isAdmin: boolean, isOwner: boolean): boolean {
  return isAdmin || isOwner;
}

export function isVehicleLendable(
  isAdmin: boolean,
  params: {
    isOwner: boolean;
    onLayout: boolean;
    dccAddress: number | null;
    leased: boolean;
  },
): boolean {
  if (isAdmin) {
    return true;
  }
  return (
    params.isOwner &&
    params.onLayout &&
    params.dccAddress != null &&
    !params.leased
  );
}

export function isTrainLendable(
  isAdmin: boolean,
  params: {
    isOwner: boolean;
    onLayout: boolean;
    leased: boolean;
  },
): boolean {
  if (isAdmin) {
    return true;
  }
  return params.isOwner && params.onLayout && !params.leased;
}

export function vehicleLendTooltip(
  t: TFunction<readonly ["vehicle", "rentals"]>,
  isAdmin: boolean,
  params: {
    onLayout: boolean;
    dccAddress: number | null;
    leased: boolean;
    isOwner: boolean;
  },
): string {
  if (isAdmin) {
    return t("rentals:granted.lend");
  }
  if (!params.isOwner) {
    return t("vehicle:catalogue.actions.lendOwnerOnly");
  }
  if (params.leased) {
    return t("vehicle:list.actions.lendAlreadyLeased");
  }
  if (!params.onLayout) {
    return t("vehicle:list.actions.lendRequiresLayout");
  }
  if (params.dccAddress == null) {
    return t("vehicle:list.actions.lendRequiresDcc");
  }
  return t("rentals:granted.lend");
}

export function trainLendTooltip(
  t: TFunction<readonly ["vehicle", "rentals"]>,
  isAdmin: boolean,
  params: {
    onLayout: boolean;
    leased: boolean;
    isOwner: boolean;
  },
): string {
  if (isAdmin) {
    return t("rentals:granted.lend");
  }
  if (!params.isOwner) {
    return t("vehicle:catalogue.actions.lendOwnerOnly");
  }
  if (params.leased) {
    return t("vehicle:trainList.actions.lendAlreadyLeased");
  }
  if (!params.onLayout) {
    return t("vehicle:trainList.actions.lendRequiresLayout");
  }
  return t("rentals:granted.lend");
}

export function isTargetLeased(
  leasedKeys: Set<string>,
  kind: "vehicle" | "train",
  targetId: string,
): boolean {
  return leasedKeys.has(lendableTargetKey({ kind, targetId }));
}
