import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const source = readFileSync(resolve("frontend/src/components/VideoChannelTab.tsx"), "utf8");
const apiSource = readFileSync(resolve("frontend/src/api.ts"), "utf8");
const handlerSource = readFileSync(resolve("internal/storespace/handler.go"), "utf8");

const checks = [
  {
    label: "frontend calls unlock api before editing confirmed channel",
    ok: /async function unlockChannelForEdit\(channel:\s*VideoChannel\)[\s\S]*storeSpaceApi\.unlockChannelForEdit\(store\.id,\s*channel\.id\)/.test(source),
  },
  {
    label: "frontend merges unlocked channel locally",
    ok: /onStoreUpdated\(\(currentStore\) => replaceChannelInStore\(currentStore,\s*updatedChannel\)\)/.test(source),
  },
  {
    label: "edit button uses unlock handler instead of local-only draft update",
    ok: /onClick=\{\(\) => void unlockChannelForEdit\(channel\)\}/.test(source),
  },
  {
    label: "api exposes unlock channel method returning channel",
    ok: /unlockChannelForEdit\(storeId:\s*number,\s*channelId:\s*number\):\s*Promise<VideoChannel>[\s\S]*storeSpaceHttpAdapter\.unlockChannelForEdit\(channelId\)/.test(apiSource),
  },
  {
    label: "backend exposes unlock route",
    ok: /POST \/api\/store-space\/channels\/\{channel_id\}\/unlock/.test(handlerSource),
  },
];

const missing = checks.filter((check) => !check.ok).map((check) => check.label);
if (missing.length > 0) {
  console.error(`Missing channel edit unlock rule(s): ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Channel edit unlock state verified.");
