import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const ezvizDomain = readFileSync(resolve("frontend/src/domain/ezviz.ts"), "utf8");
const requiredRegions = ["华北", "华东", "华南", "华中"];

const missing = requiredRegions.filter((region) => !ezvizDomain.includes(`"${region}"`));
if (missing.length > 0) {
  console.error(`Missing selectable Ezviz region(s): ${missing.join(", ")}`);
  process.exit(1);
}

console.log(`Selectable Ezviz regions verified: ${requiredRegions.join(", ")}`);
