import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The Go server serves the built app (embedded, or from disk with --ui-dir in
// development), so the access token, Host and Origin checks see one origin.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // dist/app, not dist: the build empties its folder, and dist/.gitkeep must
  // survive so the Go embed always matches something on a fresh clone.
  build: { outDir: "dist/app", emptyOutDir: true },
});
