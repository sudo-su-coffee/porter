import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// Porter ships as a single Go binary with the dashboard embedded via
// go:embed (see backend/main.go). `npm run build` here writes straight
// into ../backend/web/dist so `go build` in backend/ always bundles
// whatever was last built. During development, `npm run dev` proxies
// API calls to a locally running `porter` backend on :8080 instead.
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: "../backend/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      // Each entry is a prefix catch-all, so "/vms" also covers
      // "/vms/{id}", "/vms/{id}/logs", "/vms/{id}/domains/{domain}",
      // DELETE/PATCH verbs, etc. "/projects" likewise covers
      // "/projects/{id}" and "/projects/{id}/services/{svc}/scale".
      "/login": "http://localhost:8080",
      "/health": "http://localhost:8080",
      "/overview": "http://localhost:8080",
      "/images": "http://localhost:8080",
      "/vms": "http://localhost:8080",
      "/projects": "http://localhost:8080",
      "/events": {
        target: "http://localhost:8080",
        ws: false,
      },
    },
  },
});
