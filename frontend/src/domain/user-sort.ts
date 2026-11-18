export type UserSortInput = {
  displayName?: string;
  username?: string;
  email?: string;
};

const pinyinCollator = new Intl.Collator("zh-Hans-CN", { numeric: true, sensitivity: "base" });

export function userDisplayLabel(user: UserSortInput): string {
  return user.displayName || user.username || user.email || "";
}

export function compareUsersByPinyin(left: UserSortInput, right: UserSortInput): number {
  return pinyinCollator.compare(userDisplayLabel(left), userDisplayLabel(right));
}

export function sortUsersByPinyin<T extends UserSortInput>(users: T[]): T[] {
  return [...users].sort(compareUsersByPinyin);
}

export function userMatchesQuery(user: UserSortInput, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return [user.displayName, user.username, user.email].some((value) => (value || "").toLowerCase().includes(normalized));
}
