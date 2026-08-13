/**
 * Harbor Glass reminder: documentation should feel like the same instrument
 * as the marketing page—warm paper, graphite type, sea-glass accents, clear
 * steps, and practical detail without dashboard clutter.
 */
import { useEffect, useState, type ReactNode } from "react";
import {
  ArrowLeft,
  ArrowUpRight,
  Check,
  ChevronRight,
  Clipboard,
  ExternalLink,
  Github,
  HardDrive,
  LockKeyhole,
  Server,
  Terminal,
  Wrench,
} from "lucide-react";
import { initGA4, trackEvent, trackPageView } from "../lib/analytics";

const installCommand = "git clone https://github.com/sudo-su-coffee/porter.git && cd porter";
const runCommand = "cp backend/porter.toml.example backend/porter.toml\nmake build\n./backend/porter";
const apiExample = `curl -X POST localhost:8080/projects \\
  -H "Authorization: Bearer $PORTER_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"hello","image":"nginx:1.27"}'`;

function DocsStatusPill({ children }: { children: ReactNode }) {
  return <span className="status-pill status-pill--teal">{children}</span>;
}

function CodeBlock({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="docs-code-wrap">
      <div className="docs-code-head"><span>{label}</span><button type="button" onClick={() => { void copy(); trackEvent("docs_code_copy", { label }); }}><Clipboard size={13} />{copied ? "Copied" : "Copy"}</button></div>
      <pre><code>{value}</code></pre>
    </div>
  );
}

const checklist = [
  { icon: HardDrive, title: "A Linux host", detail: "Ubuntu or another Linux distribution with a persistent Porter state directory." },
  { icon: Server, title: "KVM + guest artifacts", detail: "`/dev/kvm`, the official Firecracker binary, and a compatible `vmlinux` plus `rootfs.ext4` pair." },
  { icon: LockKeyhole, title: "PostgreSQL", detail: "Porter stores projects, deployments, replicas, permissions, and operational history in PostgreSQL." },
  { icon: Wrench, title: "Bootstrap secrets", detail: "Set `PORTER_BOOTSTRAP_ADMIN_PASSWORD` once and keep `PORTER_SECRET_KEY` outside TOML." },
];

const deploySteps = [
  { number: "01", title: "Create a project", copy: "Use the dashboard or POST /projects with an OCI image such as nginx:1.27." },
  { number: "02", title: "Watch the replica", copy: "Porter records the deployment, allocates a replica identity, and streams state over SSE." },
  { number: "03", title: "Open the gateway", copy: "Attach a domain or use a local route. Healthy replicas receive traffic through one front door." },
];

export default function GettingStarted() {
  useEffect(() => {
    const previousTitle = document.title;
    const description = document.querySelector('meta[name="description"]');
    const previousDescription = description?.getAttribute("content") || "";
    document.title = "Getting Started — Porter";
    description?.setAttribute("content", "Install Porter on a Linux host and deploy your first app through a calm, self-hosted control plane.");
    initGA4();
    trackPageView("Getting Started — Porter");
    return () => {
      document.title = previousTitle;
      description?.setAttribute("content", previousDescription);
    };
  }, []);

  return (
    <div className="docs-shell">
      <header className="docs-header section-wrap">
        <a className="brand-lockup" href="/" aria-label="Porter home"><img className="porter-mark porter-mark--small" src="/manus-storage/porter-mark_77bf4dfd.png" alt="" aria-hidden="true" /><span>porter</span><b className="docs-brand-label">DOCS</b></a>
        <div className="docs-header-actions"><div className="docs-search"><span>Search docs</span><kbd>/</kbd></div><a href="https://github.com/sudo-su-coffee/porter" target="_blank" rel="noreferrer">Open source <ArrowUpRight size={14} /></a><a className="docs-back-link" href="/"><ArrowLeft size={15} /> Site</a></div>
      </header>

      <main>
        <div className="docs-layout section-wrap">
          <aside className="docs-sidebar">
            <div className="docs-sidebar-head"><span>PORTER DOCS</span><strong>Getting Started</strong></div>
            <div className="docs-nav-group"><span className="docs-nav-label">GET STARTED</span><a className="docs-sidebar-active" href="#requirements">Overview</a><a href="#install">Install Porter</a><a href="#configure">Configure</a><a href="#deploy">Deploy an image</a><a href="#verify">Verify the signal</a></div>
            <div className="docs-nav-group"><span className="docs-nav-label">REFERENCE</span><a href="https://github.com/sudo-su-coffee/porter/blob/main/ARCH.md" target="_blank" rel="noreferrer">Architecture <ArrowUpRight size={12} /></a><a href="https://github.com/sudo-su-coffee/porter/issues" target="_blank" rel="noreferrer">Roadmap <ArrowUpRight size={12} /></a></div>
            <div className="docs-sidebar-footer"><span className="docs-dot" /> self-hosted core</div>
          </aside>
          <div className="docs-content">
            <div className="docs-content-top"><div className="docs-breadcrumb"><a href="/">Porter</a><ChevronRight size={13} /><span>Getting Started</span></div><span className="docs-version">GUIDE / 01</span></div>
            <section id="requirements" className="docs-section">
              <div className="docs-section-kicker">Before you start</div>
              <h2>Bring a host you control.</h2>
              <p className="docs-section-lede">Porter is for builders who want a simpler PaaS without giving up the machine. Make sure the host, runtime, database, and credentials are yours to configure.</p>
              <div className="docs-checklist">{checklist.map(({ icon: Icon, title, detail }) => <article key={title}><span className="docs-check-icon"><Icon size={17} /></span><div><h3>{title}</h3><p>{detail}</p></div></article>)}</div>
            </section>

            <section id="install" className="docs-section">
              <div className="docs-section-kicker">01 / Install</div>
              <h2>Install Porter.</h2>
              <p className="docs-section-lede">Start from the public repository. Keep the first run local until the control plane is authenticated and the runtime prerequisites are confirmed.</p>
              <CodeBlock label="copy · clone the repository" value={installCommand} />
              <div className="docs-note"><Terminal size={16} /><p><strong>Compatibility first.</strong> The current repository defaults to the containerd-backed Firecracker shim. Direct Firecracker OCI boot is an explicit migration track, not a hidden assumption.</p></div>
              <CodeBlock label="copy · build and start" value={runCommand} />
            </section>

            <section id="configure" className="docs-section">
              <div className="docs-section-kicker">02 / Configure</div>
              <h2>Add your identity.</h2>
              <p className="docs-section-lede">Copy the example configuration, replace the placeholder credentials, and keep secrets out of the repository. These four values are enough to understand the first run.</p>
              <div className="docs-config-grid"><div><span>PORTER_API_TOKEN</span><p>Protects API requests and dashboard actions.</p></div><div><span>PORTER_ADMIN_PASSWORD</span><p>Authenticates the bootstrap admin session.</p></div><div><span>PORTER_DATABASE_URL</span><p>Points the store at your PostgreSQL instance.</p></div><div><span>PORTER_RUNTIME_MODE</span><p>Use containerd today; direct is the staged runtime path.</p></div></div>
              <div className="docs-warning"><LockKeyhole size={16} /><p>Do not expose the control API directly to the public internet until TLS, authentication, and host policy are configured for your environment.</p></div>
            </section>

            <section id="deploy" className="docs-section">
              <div className="docs-section-kicker">03 / Deploy</div>
              <h2>Deploy one image.</h2>
              <p className="docs-section-lede">Porter’s mental model is deliberately small: a project contains services, each service contains a desired replica pool, and every replica becomes an isolated workload with logs, health, and traffic state.</p>
              <div className="docs-deploy-steps">{deploySteps.map((step) => <article key={step.number}><span>{step.number}</span><div><h3>{step.title}</h3><p>{step.copy}</p></div></article>)}</div>
              <div className="docs-api-card"><div><span className="docs-api-label"><Github size={18} />API example</span><DocsStatusPill>REQUEST READY</DocsStatusPill></div><code>{apiExample}</code></div>
            </section>

            <section id="verify" className="docs-section">
              <div className="docs-section-kicker">04 / Verify</div>
              <h2>Confirm the signal.</h2>
              <p className="docs-section-lede">A successful first deploy is not a green button. Look for these three signals in order, then use the dashboard to inspect the path when one is missing.</p>
              <div className="docs-success-grid"><article><span>01 / CONTROL PLANE</span><strong>It responds.</strong><p>The dashboard and API load on the configured listener.</p></article><article><span>02 / REPLICA</span><strong>It is healthy.</strong><p>The workload reports a running state and a reachable address.</p></article><article><span>03 / ROUTE</span><strong>It reaches the app.</strong><p>The gateway or local route reaches the healthy replica.</p></article></div>
              <div className="docs-note docs-note--success"><Check size={16} /><p><strong>Your first win:</strong> you can explain the path from image to boundary to healthy response without opening a second platform.</p></div>
            </section>

            <section id="operate" className="docs-section docs-section--last">
              <div className="docs-section-kicker">Next / Operate with intent</div>
              <h2>Ship the next one.</h2>
              <p className="docs-section-lede">Once the first replica is healthy, explore the parts that make Porter useful as a platform: domains and TLS, health-aware routing, deployment history, logs, metrics, traffic, volumes, RBAC, and the direct-runtime migration path.</p>
              <div className="docs-next-grid"><a href="/#how-it-works"><span>Product flow <ArrowUpRight size={15} /></span><p>See how a change becomes a running boundary.</p></a><a href="https://github.com/sudo-su-coffee/porter/blob/main/ARCH.md" target="_blank" rel="noreferrer"><span>Architecture <ArrowUpRight size={15} /></span><p>Read the current project and replica mental model.</p></a><a href="https://github.com/sudo-su-coffee/porter/issues" target="_blank" rel="noreferrer"><span>Roadmap <ArrowUpRight size={15} /></span><p>Follow the open work before the hosted SaaS layer.</p></a></div>
            </section>
          </div>
        </div>
      </main>

      <footer className="site-footer section-wrap"><a className="brand-lockup" href="/"><img className="porter-mark porter-mark--small" src="/manus-storage/porter-mark_77bf4dfd.png" alt="" aria-hidden="true" /><span>porter</span></a><span>self-hosted deployment, without the ceremony</span><a href="/#top">Back to site <ArrowUpRight size={14} /></a></footer>
    </div>
  );
}
