import type { UserSortInput } from "./user-sort";

export type OperatorCandidate = UserSortInput & { id: number };

export type AuditActorInput = {
  userId?: number | null;
  actorDisplayName?: string;
  userEmail?: string;
};

export type OperatorNameCandidate = {
  key: string;
  displayName: string;
  email: string;
};

function actorKey(actor: AuditActorInput): string {
  const email = (actor.userEmail || "").trim().toLowerCase();
  if (email) return `email:${email}`;
  const name = (actor.actorDisplayName || "").trim();
  return name ? `name:${name}` : "";
}

// Historical audit rows may lack user_id; keep those operators searchable by name/email.
// Such candidates filter by name instead of id and are marked as approximate matches.
export function buildOperatorNameCandidates(actors: AuditActorInput[], users: (UserSortInput & { id: number })[]): OperatorNameCandidate[] {
  const userIds = new Set(users.map((user) => user.id));
  const userEmails = new Set(users.map((user) => (user.email || "").trim().toLowerCase()).filter(Boolean));
  const userNames = new Set(users.map((user) => (user.displayName || "").trim()).filter(Boolean));
  const byKey = new Map<string, OperatorNameCandidate>();
  for (const actor of actors) {
    if (actor.userId != null && userIds.has(actor.userId)) continue;
    const email = (actor.userEmail || "").trim().toLowerCase();
    const name = (actor.actorDisplayName || "").trim();
    if (email && userEmails.has(email)) continue;
    if (!email && name && userNames.has(name)) continue;
    const key = actorKey(actor);
    if (!key || byKey.has(key)) continue;
    byKey.set(key, { key, displayName: name, email: (actor.userEmail || "").trim() });
  }
  return [...byKey.values()];
}

export function buildOperatorOptions(users: OperatorCandidate[], actors: AuditActorInput[]): OperatorCandidate[] {
  const byId = new Map<number, OperatorCandidate>();
  for (const user of users) byId.set(user.id, user);
  for (const actor of actors) {
    if (actor.userId == null || byId.has(actor.userId)) continue;
    byId.set(actor.userId, {
      id: actor.userId,
      displayName: actor.actorDisplayName || "",
      username: "",
      email: actor.userEmail || "",
    });
  }
  return [...byId.values()];
}
