export interface UserNameInput {
  login?: string;
  ownerLogin?: string;
  organization?: string | null;
  ownerOrganization?: string | null;
}

/** Full display name: `[organization] login`, or plain login when organization is empty. */
export function getUserName(user: UserNameInput): string {
  const login = (user.login ?? user.ownerLogin ?? "").trim();
  const organization = (user.organization ?? user.ownerOrganization ?? "").trim();
  if (!organization) {
    return login;
  }
  return `[${organization}] ${login}`;
}
