import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const apiSource = readFileSync(resolve("frontend/src/api.ts"), "utf8");

const checks = [
  {
    label: "stored preview path conversion",
    pattern: /uploads\\\/\(\[\^\/]\+\)\\\/\(preview\|thumbnail\)\\\.png/,
  },
  {
    label: "stored preview path maps to design-plan upload endpoint",
    pattern: /\/api\/design-plan\/uploads\/\$\{[^}]+}\//,
  },
];

const missing = checks.filter((check) => !check.pattern.test(apiSource)).map((check) => check.label);

if (missing.length > 0) {
  console.error(`Missing design-plan image URL conversion: ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Design-plan image URL conversion verified.");
