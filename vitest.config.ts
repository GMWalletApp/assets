import path from "node:path";
import { defineConfig } from "vitest/config";

const root = import.meta.dirname;

export default defineConfig({
  resolve: {
    alias: {
      fzstd: path.resolve(root, "tests/fzstd.ts"),
      "@/components/ui/avatar": path.resolve(root, "tests/registry/avatar.tsx"),
      "@/components/ui/skeleton": path.resolve(root, "tests/registry/skeleton.tsx"),
      "@/lib/utils": path.resolve(root, "tests/registry/utils.ts"),
      "@": path.resolve(root, "src"),
    },
  },
  test: {
    coverage: {
      reporter: ["text", "json", "html"],
    },
  },
});
