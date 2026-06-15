import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/components/VideoChannelTab.tsx"), "utf8");

const checks = [
  {
    label: "confirm channel updates row before api resolves",
    ok: /const optimisticChannel = confirmedChannelDraft[\s\S]*onStoreUpdated\(\(currentStore\) => replaceChannelInStore\(currentStore,\s*optimisticChannel\)\)/.test(source),
  },
  {
    label: "unlock channel updates row before api resolves",
    ok: /const optimisticChannel = \{ \.\.\.channel,\s*status:\s*"pending_confirmation"[\s\S]*onStoreUpdated\(\(currentStore\) => replaceChannelInStore\(currentStore,\s*optimisticChannel\)\)/.test(source),
  },
  {
    label: "delete channel removes row before api resolves",
    ok: /onStoreUpdated\(\(currentStore\) => removeChannelFromStore\(currentStore,\s*channel\.id\)\)/.test(source),
  },
  {
    label: "delete recorder removes row before api resolves",
    ok: /onStoreUpdated\(\(currentStore\) => removeRecorderFromStore\(currentStore,\s*recorder\.id\)\)/.test(source),
  },
  {
    label: "failed optimistic action restores previous store",
    ok: /onStoreUpdated\(previousStore\)/.test(source),
  },
];

const missing = checks.filter((check) => !check.ok).map((check) => check.label);
if (missing.length > 0) {
  console.error(`Missing channel optimistic action rule(s): ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Channel optimistic action state verified.");
