import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/components/VideoChannelTab.tsx"), "utf8");

const requiredPatterns = [
  {
    label: "single channel recognition merges into latest store state",
    pattern: /onStoreUpdated\(\(currentStore\) => replaceChannelInStore\(currentStore,\s*updatedChannel\)\)/,
  },
];

const forbiddenPatterns = [
  {
    label: "single channel recognition uses stale store prop",
    pattern: /const\s+nextStore\s*=\s*replaceChannelInStore\(store,\s*updatedChannel\)/,
  },
];

const missing = requiredPatterns.filter((check) => !check.pattern.test(source)).map((check) => check.label);
const forbidden = forbiddenPatterns.filter((check) => check.pattern.test(source)).map((check) => check.label);

if (missing.length > 0 || forbidden.length > 0) {
  if (missing.length > 0) console.error(`Missing channel recognition merge rule(s): ${missing.join(", ")}`);
  if (forbidden.length > 0) console.error(`Forbidden channel recognition merge rule(s): ${forbidden.join(", ")}`);
  process.exit(1);
}

console.log("Channel recognition merge state verified.");
