import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/erzhuang-project/",
  plugins: [react()],
  test: {
    include: [
      "src/api.test.ts",
      "src/components/**/*.test.{ts,tsx}",
      "src/pages/**/*.test.{ts,tsx}",
      "src/domain/channel-recognition.test.ts",
      "src/domain/h5-monitor-active-tab.test.ts",
      "src/domain/nvr-lab.test.ts",
      "src/domain/resource-view.test.ts",
    ],
    env: {
      VITE_DESIGN_PLAN_API_BASE: "/erzhuang-project/api/design-plan",
      VITE_STORE_SPACE_API_BASE: "/erzhuang-project/api/store-space",
    },
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/erzhuang-project/api": {
        target: "http://127.0.0.1:18080",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/erzhuang-project\/api/, "/api"),
      },
    },
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
  },
});
