import { createRequire } from "node:module";

const require = createRequire(new URL("../../frontend/package.json", import.meta.url));
const express = require("express");

const app = express();
const port = Number(process.env.PORTER_MOCK_PORT || 8787);
app.use(express.json());

const projects = [
  { id: "proj-api", name: "porter-api", kind: "image", image: "ghcr.io/porter/api:dev", status: "running", network: "10.42.0.0/24", replicas_desired: 2, tags: ["control-plane", "go"] , created_at: "2026-08-14T08:20:00Z" },
  { id: "proj-docs", name: "porter-docs", kind: "compose", image: "—", status: "stopped", network: "10.42.1.0/24", replicas_desired: 1, tags: ["docs", "preview"], created_at: "2026-08-12T12:00:00Z" },
];
const replicas = [
  { id: "vm-api-1", project_id: "proj-api", name: "porter-api-1", state: "running", health_status: "healthy", ip_address: "10.42.0.11", image: "ghcr.io/porter/api:dev" },
  { id: "vm-api-2", project_id: "proj-api", name: "porter-api-2", state: "running", health_status: "healthy", ip_address: "10.42.0.12", image: "ghcr.io/porter/api:dev" },
  { id: "vm-docs-1", project_id: "proj-docs", name: "porter-docs-1", state: "stopped", health_status: "unknown", ip_address: "10.42.1.11", image: "docs:preview" },
];
const deployments = [
  { id: "dep-104", project_id: "proj-api", revision: 104, build_status: "ready", rollout_percent: 100, image_digest: "sha256:91e0…", created_at: "2026-08-14T09:10:00Z" },
  { id: "dep-103", project_id: "proj-api", revision: 103, build_status: "rolled_back", rollout_percent: 0, image_digest: "sha256:713a…", created_at: "2026-08-13T16:42:00Z" },
  { id: "dep-7", project_id: "proj-docs", revision: 7, build_status: "ready", rollout_percent: 100, image_digest: "sha256:2ac1…", created_at: "2026-08-12T12:06:00Z" },
];
const logs = [
  { id: "log-1", level: "info", message: "replica vm-api-2 passed health check", created_at: "2026-08-14T09:22:12Z" },
  { id: "log-2", level: "debug", message: "deployment dep-104 reconciled at 100%", created_at: "2026-08-14T09:20:41Z" },
];

function auth(req, res, next) {
  if (req.path === "/login" || req.path === "/csrf" || req.path === "/health") return next();
  res.setHeader("X-Request-Id", `mock-${Date.now()}`);
  return next();
}
app.use(auth);

app.get("/health", (_req, res) => res.json({ status: "ok", version: "dev-mock" }));
app.post("/login", (_req, res) => res.json({ token: "dev-preview-token" }));
app.post("/auth/login", (_req, res) => res.json({ token: "dev-preview-token" }));
app.get("/csrf", (_req, res) => res.json({ csrf_token: "dev-preview-csrf" }));
app.get("/overview", (_req, res) => res.json({ projects: projects.length, vm_total: replicas.length, vm_running: replicas.filter((v) => v.state === "running").length, started_at: "2026-08-14T08:00:00Z" }));
app.get("/projects", (_req, res) => res.json(projects));
app.get("/projects/:id", (req, res) => {
  const project = projects.find((p) => p.id === req.params.id);
  if (!project) return res.status(404).json({ error: "Project not found" });
  return res.json({ project, vms: replicas.filter((v) => v.project_id === project.id) });
});
app.get("/projects/:id/deployments", (req, res) => res.json(deployments.filter((d) => d.project_id === req.params.id)));
app.get("/projects/:id/traffic", (_req, res) => res.json([{ window: "5m", requests: 1842, errors: 3, p95_ms: 42 }]));
app.get("/projects/:id/logs", (_req, res) => res.json(logs));
app.get("/projects/:id/domains", (_req, res) => res.json([{ id: "domain-1", hostname: "api.porter.local", status: "active" }]));
app.get("/projects/:id/env", (_req, res) => res.json([{ key: "PORTER_ENV", value: "development" }, { key: "LOG_LEVEL", value: "debug" }]));
app.get("/deployments", (_req, res) => res.json(deployments));
app.get("/replicas", (_req, res) => res.json(replicas));
app.get("/vms", (_req, res) => res.json({ vms: replicas }));
app.get("/images", (_req, res) => res.json([{ id: "base-default", name: "default", kind: "base", status: "ready" }]));
app.get("/servers", (_req, res) => res.json([{ id: "host-local", name: "PC", status: "ready", runtime: "direct-firecracker" }]));
app.get("/logs", (_req, res) => res.json(logs));
app.get("/events", (_req, res) => {
  res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-cache", Connection: "keep-alive" });
  res.write(`event: connected\\ndata: ${JSON.stringify({ source: "dev-mock" })}\\n\\n`);
  const timer = setInterval(() => res.write(`event: heartbeat\\ndata: ${JSON.stringify({ at: new Date().toISOString() })}\\n\\n`), 10000);
  res.on("close", () => clearInterval(timer));
});
app.all("*", (req, res) => res.status(200).json({ data: [], endpoint: req.path, method: req.method, preview: true }));

app.listen(port, "127.0.0.1", () => console.log(`Porter mock API listening on http://127.0.0.1:${port}`));
