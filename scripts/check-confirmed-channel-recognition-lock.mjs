import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/components/VideoChannelTab.tsx"), "utf8");

const requiredPatterns = [
  {
    label: "confirmed channel helper exists",
    pattern: /function isConfirmedChannel\(channel:\s*VideoChannel\)/,
  },
  {
    label: "recorder recognition skips confirmed channels",
    pattern: /recorder\.channels\.filter\(\s*\(channel\) => channel\.status !== "inactive" && !isConfirmedChannel\(channel\)\s*\)/,
  },
  {
    label: "single channel recognition button locks confirmed channels",
    pattern: /disabled=\{isRecognizing \|\| isDeleting \|\| isConfirmed \|\| workingRecorderId === recorder\.id\}/,
  },
  {
    label: "edit action unlocks channel to pending confirmation",
    pattern: /updateChannelDraft\(channel\.id,\s*\{\s*status:\s*"pending_confirmation"\s*\}\)/,
  },
];

const missing = requiredPatterns.filter((check) => !check.pattern.test(source)).map((check) => check.label);

if (missing.length > 0) {
  console.error(`Missing confirmed channel recognition lock rule(s): ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Confirmed channel recognition lock verified.");
