import type { CurrentUser } from "../api/auth";

/** Permanent or sudo admin powers for layout roster actions. */
export function hasEffectiveAdmin(me: CurrentUser | null | undefined): boolean {
  if (!me) {
    return false;
  }
  return me.effectiveRole === "admin" || me.sudo != null;
}

export function canRemoveFromLayout(
  me: CurrentUser | null | undefined,
  ownerId: number,
): boolean {
  return me?.id === ownerId || hasEffectiveAdmin(me);
}

export function canAddToLayout(
  me: CurrentUser | null | undefined,
  ownerId: number,
): boolean {
  return me?.id === ownerId || hasEffectiveAdmin(me);
}
