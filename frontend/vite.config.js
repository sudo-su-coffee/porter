import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// Porter ships as a single Go binary with the dashboard embedded via
// go:embed (see backend/main.go). `npm run build` here writes straight
// into ../backend/web/dist so `go build` in backend/ always bundles
// whatever was last built. During development, `npm run dev` proxies
// API calls to a locally running `porter` backend on :8080 instead.
const apiTarget = process.env.VITE_PORTER_PREVIEW === "true" ? "http://127.0.0.1:8787" : "http://localhost:8080";

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
      "/auth/login": apiTarget,
      "/login": apiTarget,
      "/csrf": apiTarget,
      "/health": apiTarget,
      "/overview": apiTarget,
      "/images": apiTarget,
      "/logs": apiTarget,
      "/servers": apiTarget,
      "/volumes": apiTarget,
      "/vms": apiTarget,
      "/projects": apiTarget,
      "/deployments": apiTarget,
      "/replicas": apiTarget,
      "/events": {
        target: apiTarget,
        ws: false,
      },
    },
  },
});
