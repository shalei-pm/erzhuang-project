import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/erzhuang/",
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/erzhuang/api": {
        target: "http://127.0.0.1:18080",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/erzhuang\/api/, "/api"),
      },
    },
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
  },
});
