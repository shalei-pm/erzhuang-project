import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/styles.css"), "utf8");

const match = source.match(/\.row-actions \.danger-link:hover:not\(:disabled\),\s*\.danger-link:hover:not\(:disabled\)\s*\{([^}]*)\}/);

if (!match || !/color:\s*var\(--danger\);/.test(match[1])) {
  console.error("Danger delete buttons must set red text color on hover.");
  process.exit(1);
}

console.log("Danger link hover color verified.");
