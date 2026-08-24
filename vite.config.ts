import { fileURLToPath, URL } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const baseUrl = process.env.PAGES_BASE_PATH ?? "/";
export default defineConfig({
  base: baseUrl,
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
    tsconfigPaths: true,
  },
  plugins: [
    tailwindcss(),
    tanstackStart({
      pages: [{ path: "/" }, { path: "/usage" }],
      prerender: {
        enabled: true,
        autoStaticPathsDiscovery: false,
        crawlLinks: false,
        failOnError: true,
      },
    }),
    viteReact(),
  ],
});
