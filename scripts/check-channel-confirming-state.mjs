import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/components/VideoChannelTab.tsx"), "utf8");

const requiredPatterns = [
  {
    label: "confirming channels use an id set",
    pattern: /confirmingChannelIds,\s*setConfirmingChannelIds[\s\S]*useState<Set<number>>/,
  },
  {
    label: "confirm start adds only current channel",
    pattern: /setConfirmingChannelIds\(\(current\) => addIdToSet\(current,\s*channel\.id\)\)/,
  },
  {
    label: "confirm finish removes only current channel",
    pattern: /setConfirmingChannelIds\(\(current\) => removeIdFromSet\(current,\s*channel\.id\)\)/,
  },
  {
    label: "row reads confirming state per channel",
    pattern: /const isConfirming = confirmingChannelIds\.has\(channel\.id\)/,
  },
];

const forbiddenPatterns = [
  {
    label: "single confirming channel state",
    pattern: /\bconfirmingChannelId\b|\bsetConfirmingChannelId\b/,
  },
];

const missing = requiredPatterns.filter((check) => !check.pattern.test(source)).map((check) => check.label);
const forbidden = forbiddenPatterns.filter((check) => check.pattern.test(source)).map((check) => check.label);

if (missing.length > 0 || forbidden.length > 0) {
  if (missing.length > 0) console.error(`Missing channel confirming state rule(s): ${missing.join(", ")}`);
  if (forbidden.length > 0) console.error(`Forbidden channel confirming state rule(s): ${forbidden.join(", ")}`);
  process.exit(1);
}

console.log("Channel confirming state verified.");
