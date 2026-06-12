import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const appSource = readFileSync(resolve("frontend/src/App.tsx"), "utf8");
const listSource = readFileSync(resolve("frontend/src/components/StoreList.tsx"), "utf8");

const checks = [
  {
    label: "app tracks opening store ids",
    ok: /openingStoreIds,\s*setOpeningStoreIds[\s\S]*useState<Set<number>>/.test(appSource),
  },
  {
    label: "open store marks request loading",
    ok: /setOpeningStoreIds\(\(current\) => new Set\(current\)\.add\(storeId\)\)/.test(appSource),
  },
  {
    label: "open store ignores duplicate click while loading",
    ok: /if \(openingStoreIds\.has\(storeId\)\) return;/.test(appSource),
  },
  {
    label: "open store clears request loading",
    ok: /setOpeningStoreIds\(\(current\) => removeIdFromSet\(current,\s*storeId\)\)/.test(appSource),
  },
  {
    label: "store list disables opening row",
    ok: /disabled=\{isDeleting \|\| isOpening\}/.test(listSource),
  },
  {
    label: "store list shows entering label",
    ok: /进入中/.test(listSource),
  },
];

const missing = checks.filter((check) => !check.ok).map((check) => check.label);
if (missing.length > 0) {
  console.error(`Missing store open loading rule(s): ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Store open loading state verified.");
