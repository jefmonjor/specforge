import react from "@vitejs/plugin-react";
import { configDefaults, defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    // Stryker's sandboxes hold copies of the tests.
    exclude: [...configDefaults.exclude, ".stryker-tmp/**", "reports/**"],
  },
});
