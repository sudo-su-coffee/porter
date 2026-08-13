import fs from "node:fs";
import path from "node:path";

const repo = path.resolve(path.dirname(new URL(import.meta.url).pathname), "../..");
const apiFile = path.join(repo, "backend/internal/api/api.go");
const frontendRoot = path.join(repo, "frontend/src");
const reportFile = path.join(repo, "docs/frontend/API_ENDPOINT_VIEW_COVERAGE.md");

function filesUnder(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...filesUnder(full));
    else if (/\.(js|vue)$/.test(entry.name)) out.push(full);
  }
  return out;
}

function normalize(value) {
  return value
    .replace(/\$\{[^}]*\}/g, ":param")
    .replace(/\{[^}]+\}/g, ":param")
    .replace(/:[A-Za-z_][A-Za-z0-9_]*/g, ":param")
    .replace(/\?.*$/, "")
    .replace(/\/+/g, "/")
    .replace(/\/$/, "") || "/";
}

const apiSource = fs.readFileSync(apiFile, "utf8");
const routes = [...apiSource.matchAll(/mux\.HandleFunc\("([A-Z]+)\s+([^"\\]+)"/g)].map((match) => ({
  method: match[1],
  path: match[2],
  normalized: normalize(match[2]),
}));

const sourceFiles = filesUnder(frontendRoot);
const endpointEvidence = new Map();
for (const file of sourceFiles) {
  const source = fs.readFileSync(file, "utf8");
  const relative = path.relative(repo, file);
  const literals = [...source.matchAll(/["'`](\/[^"'`\r\n]*)["'`]/g)].map((match) => match[1]);
  for (const literal of literals) {
    const normalized = normalize(literal);
    if (!endpointEvidence.has(normalized)) endpointEvidence.set(normalized, new Set());
    endpointEvidence.get(normalized).add(relative);
  }
}

const rows = routes.map((route) => ({ ...route, evidence: [...(endpointEvidence.get(route.normalized) || [])] }));
const missing = rows.filter((row) => row.evidence.length === 0);
const exceptionPrefixes = [
  "/health", "/healthz", "/version", "/csrf", "/auth", "/login", "/logout", "/events",
];
const intentional = missing.filter((row) => exceptionPrefixes.some((prefix) => row.path === prefix || row.path.startsWith(`${prefix}/`)));
const productMissing = missing.filter((row) => !intentional.includes(row));

const lines = [
  "# API endpoint to Vue source coverage report",
  "",
  `Generated from backend/internal/api/api.go and frontend/src on ${new Date().toISOString()}.`,
  "",
  "| Measure | Count |",
  `|---|---:|`,
  `| Registered api.go routes | ${rows.length} |`,
  `| Routes with normalized Vue source evidence | ${rows.length - missing.length} |`,
  `| Routes without literal source evidence | ${missing.length} |`,
  `| Unmatched documented transport/auth prefixes | ${intentional.length} |`,
  `| Unmatched product routes requiring review | ${productMissing.length} |`,
  "",
  "> This is a deterministic path-evidence report, not a substitute for the method/payload review. Resource schemas and shared components are included because their endpoint strings are declared in the router or component source.",
  "",
  "## Product routes requiring review",
  "",
  "| Method | Backend path | Normalized path | Source evidence |",
  "|---|---|---|---|",
];
for (const row of productMissing) lines.push("| " + row.method + " | " + row.path + " | " + row.normalized + " | **missing** |");
if (!productMissing.length) lines.push("| — | — | — | No unmatched product route paths detected by the normalized source scan. |");
lines.push("", "## Covered route evidence", "", "| Method | Backend path | Vue source files |", "|---|---|---|");
for (const row of rows) {
  const evidence = row.evidence.length ? row.evidence.join(", ") : "documented transport/auth exception";
  lines.push("| " + row.method + " | " + row.path + " | " + evidence + " |");
}

fs.writeFileSync(reportFile, `${lines.join("\n")}\n`);
console.log(JSON.stringify({ routes: rows.length, matched: rows.length - missing.length, missing: missing.length, intentional: intentional.length, productMissing: productMissing.length, reportFile }, null, 2));
