/**
 * Harbor Glass reminder: warm mineral surfaces, graphite type, sea-glass teal,
 * quiet glass panels, asymmetric editorial composition, and motion that feels
 * like a physical instrument rather than a neon dashboard.
 */
import { useEffect, useState, type ReactNode } from "react";
import {
  Activity,
  ArrowRight,
  ArrowUpRight,
  Boxes,
  Check,
  Cloud,
  CircleDot,
  Code2,
  Gauge,
  GitBranch,
  Globe2,
  LockKeyhole,
  Menu,
  Network,
  Users,
  Server,
  ShieldCheck,
  TerminalSquare,
  X,
} from "lucide-react";
import { initGA4, trackEvent, trackPageView } from "../lib/analytics";

const HERO_IMAGE = "/manus-storage/porter-hero-atmosphere_20783e89.png";
const FLOW_IMAGE = "/manus-storage/porter-deploy-flow_0f310b35.png";
const MARK_IMAGE = "/manus-storage/porter-mark_77bf4dfd.png";

const navItems = [
  { label: "Product", href: "#product" },
  { label: "Docs", href: "/getting-started" },
];

const flowSteps = [
  {
    number: "01",
    title: "Choose the source",
    description: "Point Porter at an OCI image, a Compose file, or a Git repository.",
    icon: Code2,
  },
  {
    number: "02",
    title: "Build the boundary",
    description: "Each replica gets its own microVM, network identity, and health signal.",
    icon: ShieldCheck,
  },
  {
    number: "03",
    title: "Route the traffic",
    description: "One gateway handles domains, TLS, logs, metrics, and healthy replicas.",
    icon: Network,
  },
  {
    number: "04",
    title: "Keep the signal",
    description: "See health, events, and traffic after the deploy so the next decision stays clear.",
    icon: Activity,
  },
];

function PorterMark({ size = "small" }: { size?: "small" | "large" }) {
  return (
    <img
      className={`porter-mark porter-mark--${size}`}
      src={MARK_IMAGE}
      alt=""
      aria-hidden="true"
    />
  );
}

function StatusPill({ children, tone = "teal" }: { children: ReactNode; tone?: "teal" | "apricot" }) {
  return <span className={`status-pill status-pill--${tone}`}><CircleDot size={11} strokeWidth={2.5} />{children}</span>;
}

export default function Home() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    initGA4();
    trackPageView("Porter — Home");
    const onScroll = () => setScrolled(window.scrollY > 24);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <div className="site-shell">
      <header className={`site-header ${scrolled ? "site-header--scrolled" : ""}`}>
        <a className="brand-lockup" href="#top" aria-label="Porter home">
          <PorterMark />
          <span>porter</span>
        </a>
        <nav className={`desktop-nav ${menuOpen ? "desktop-nav--open" : ""}`} aria-label="Primary navigation">
          {navItems.map((item) => (
            <a key={item.label} href={item.href} onClick={() => setMenuOpen(false)}>{item.label}</a>
          ))}
          <a className="nav-cta" href="/getting-started#install" onClick={() => { setMenuOpen(false); trackEvent("first_deploy_cta_click", { placement: "nav" }); }}>Start your first deploy <ArrowUpRight size={15} /></a>
        </nav>
        <button className="menu-toggle" type="button" aria-label={menuOpen ? "Close menu" : "Open menu"} aria-expanded={menuOpen} onClick={() => setMenuOpen((open) => !open)}>
          {menuOpen ? <X size={21} /> : <Menu size={21} />}
        </button>
      </header>

      <main id="top">
        <section className="hero-section section-wrap">
          <div className="hero-copy">
            <div className="eyebrow"><span className="eyebrow-rule" />Self-hosted deployment</div>
            <h1>The platform<br /><em>that stays yours.</em></h1>
            <p className="hero-lede">Deploy from one calm control plane. Give every replica a real boundary. Keep your data, your host, and your decisions close.</p>
            <div className="hero-actions">
              <a className="button button--primary" href="/getting-started#install" onClick={() => trackEvent("first_deploy_cta_click", { placement: "hero" })}>Start with one app <ArrowRight size={17} /></a>
              <a className="button button--quiet" href="/getting-started#deploy">See how it works <ArrowUpRight size={16} /></a>
            </div>
            <div className="home-guide-card"><div><span>GETTING STARTED</span><strong>From host to healthy.</strong><p>One short path to your first Porter-managed app.</p></div><a href="/getting-started#install" aria-label="Open Getting Started" onClick={() => trackEvent("getting_started_open", { placement: "hero_card" })}><ArrowRight size={17} /></a></div>
          </div>

          <div className="hero-instrument glass-panel">
            <img className="hero-atmosphere" src={HERO_IMAGE} alt="Abstract sea-glass deployment path" />
            <div className="instrument-topline"><span>PORTER / CONTROL PLANE</span><StatusPill>HOST ONLINE</StatusPill></div>
            <div className="instrument-heading"><span className="instrument-kicker">CURRENT DEPLOYMENT</span><strong>storefront-api</strong><span className="instrument-subline">release / 7c4f19a · eu-west-1</span></div>
            <div className="route-visual" aria-label="Deployment route from source to replicas">
              <div className="route-line" />
              <div className="route-node route-node--source"><GitBranch size={17} /><span>git push</span></div>
              <div className="route-node route-node--build"><Boxes size={17} /><span>image</span></div>
              <div className="route-node route-node--vm"><Server size={17} /><span>microVM</span></div>
            </div>
            <div className="instrument-grid">
              <div><span>REPLICAS</span><strong>03 / 03</strong></div>
              <div><span>HEALTH</span><strong className="value-teal">99.98%</strong></div>
              <div><span>BOOT</span><strong>1.42s</strong></div>
            </div>
            <div className="instrument-footer"><StatusPill tone="apricot">DEPLOYED 38s AGO</StatusPill><span>view event stream <ArrowUpRight size={14} /></span></div>
          </div>
        </section>

        <section className="signal-strip section-wrap" aria-label="Porter product promises">
          <div><span className="signal-index">01</span><strong>ONE CONTROL PLANE</strong><span>see the whole path</span></div>
          <div><span className="signal-index">02</span><strong>ONE REAL BOUNDARY</strong><span>per replica, by design</span></div>
          <div><span className="signal-index">03</span><strong>ONE HOST YOU OWN</strong><span>open source · self-hosted</span></div>
        </section>

        <section id="product" className="product-section section-wrap">
          <div className="section-intro">
            <div className="eyebrow"><span className="eyebrow-rule" />The useful middle</div>
            <h2>Simple to start.<br /><em>Serious underneath.</em></h2>
            <p>Porter sits between “just SSH into the box” and “operate a cluster.” It gives your app a real lifecycle without giving the host away.</p>
          </div>
          <div className="feature-stack">
            <article className="feature-card feature-card--wide">
              <div className="feature-icon"><ShieldCheck size={20} /></div>
              <div><span className="card-label">01 / ISOLATE</span><h3>Every replica gets a boundary.</h3><p>Run workloads with Firecracker-level isolation while keeping the operator experience closer to a familiar PaaS.</p></div>
              <span className="card-corner">/01</span>
            </article>
            <article className="feature-card feature-card--offset">
              <div className="feature-icon"><Activity size={20} /></div>
              <div><span className="card-label">02 / OBSERVE</span><h3>See the path from push to healthy.</h3><p>Deployments, replica health, logs, traffic, and events share one surface.</p></div>
              <span className="card-corner">/02</span>
            </article>
            <article className="feature-card feature-card--wide feature-card--soft">
              <div className="feature-icon"><Globe2 size={20} /></div>
              <div><span className="card-label">03 / ROUTE</span><h3>Put one clear front door in front of the pool.</h3><p>Domains, TLS, health-aware routing, and traffic records live alongside the deploy instead of in another tool.</p></div>
              <span className="card-corner">/03</span>
            </article>
          </div>
        </section>

        <section id="how-it-works" className="flow-section section-wrap">
          <div className="flow-heading">
            <div><div className="eyebrow"><span className="eyebrow-rule" />How it works</div><h2>A deploy you<br /><em>can follow.</em></h2></div>
            <p>Your code enters once. Porter keeps every important handoff visible—from the first push to the first healthy response.</p>
          </div>
          <div className="flow-stage glass-panel">
            <div className="flow-art"><img src={FLOW_IMAGE} alt="Abstract glass deployment flow" /><div className="flow-art-caption"><TerminalSquare size={15} /> porter deploy / storefront-api</div></div>
            <div className="flow-list">
              <div className="flow-list-top"><span>DEPLOYMENT / 01</span><strong>Every handoff is visible.</strong></div>
              {flowSteps.map((step) => {
                const Icon = step.icon;
                return <div className="flow-step" key={step.number}><span className="flow-number">{step.number}</span><span className="flow-step-icon"><Icon size={17} /></span><div><h3>{step.title}</h3><p>{step.description}</p></div></div>;
              })}
              <a className="flow-link" href="/getting-started#deploy">Walk through a first deploy <ArrowRight size={14} /></a>
            </div>
          </div>
        </section>

        <section className="capability-section section-wrap">
          <div className="capability-heading"><div><div className="eyebrow"><span className="eyebrow-rule" />Built for the whole lifecycle</div><h2>Everything you need.<br /><em>Nothing you don’t.</em></h2></div><p>Porter gives deployments, boundaries, and signals one home—without hiding the host from the person operating it.</p></div>
          <div className="signature-rail" aria-label="Porter operating path"><span><CircleDot size={12} /> source</span><i /><span><CircleDot size={12} /> boundary</span><i /><span><CircleDot size={12} /> route</span><i /><span><CircleDot size={12} /> health</span></div>
          <div className="capability-grid">
            <article className="capability-card capability-card--large"><div className="capability-card-top"><span className="feature-icon"><Gauge size={19} /></span><StatusPill>AVAILABLE NOW</StatusPill></div><span className="card-label">01 / OPERATE</span><h3>Deployments that explain themselves.</h3><p>Follow a release from source to image to healthy replica with logs, health, traffic, and events in the same control plane.</p><div className="capability-meter"><span /><span /><span /><span /></div><small>deployment / replica / gateway / signal</small></article>
            <article className="capability-card"><span className="feature-icon"><LockKeyhole size={19} /></span><span className="card-label">02 / PROTECT</span><h3>Boundaries you can reason about.</h3><p>Firecracker microVMs, project-level networks, domains, TLS, and role-aware access as the platform grows.</p><div className="capability-stat"><strong>1</strong><span>clear workload boundary per replica</span></div></article>
            <article className="capability-card"><span className="feature-icon"><Users size={19} /></span><span className="card-label">03 / COLLABORATE</span><h3>A better path for teams.</h3><p>Keep the self-hosted core open while a future hosted workspace adds shared access, usage visibility, and support.</p><div className="capability-stat"><strong>∞</strong><span>room to grow from one host to a team</span></div></article>
          </div>
        </section>

        <section id="saas" className="saas-section section-wrap">
          <div className="saas-heading"><div className="eyebrow"><span className="eyebrow-rule" />A path, not a lock-in</div><h2>Start open.<br /><em>Grow into a platform.</em></h2><p>Porter can earn trust before it earns a subscription. The product path starts with an open self-hosted core, then adds convenience where teams feel the operational weight.</p></div>
          <div className="plan-rail">
            <article className="plan-card plan-card--active"><div className="plan-card-head"><span>SELF-HOSTED</span><StatusPill>AVAILABLE</StatusPill></div><strong className="plan-price">$0 <small>software</small></strong><p>Run Porter on your own host with full control over the runtime and data.</p><div className="plan-points"><span><Check size={14} />Open source core</span><span><Check size={14} />Your infrastructure</span><span><Check size={14} />Direct ownership</span></div><a href="/getting-started" className="plan-link">Get started <ArrowRight size={15} /></a></article>
            <article className="plan-card"><div className="plan-card-head"><span>HOSTED WORKSPACE</span><StatusPill tone="apricot">PLANNED</StatusPill></div><strong className="plan-price">Team <small>when ready</small></strong><p>A managed control plane for teams that want collaboration without managing the dashboard host.</p><div className="plan-points"><span><Check size={14} />Shared workspaces</span><span><Check size={14} />Usage + spend view</span><span><Check size={14} />Managed updates</span></div><a href="https://github.com/sudo-su-coffee/porter" target="_blank" rel="noreferrer" className="plan-link">Follow the build <ArrowUpRight size={15} /></a></article>
            <article className="plan-card"><div className="plan-card-head"><span>PRIVATE FLEET</span><StatusPill tone="apricot">PLANNED</StatusPill></div><strong className="plan-price">Custom <small>for serious teams</small></strong><p>Support, policy, and fleet controls for organizations that need a private deployment posture.</p><div className="plan-points"><span><Check size={14} />SSO + policy</span><span><Check size={14} />Priority support</span><span><Check size={14} />Fleet visibility</span></div><a href="https://github.com/sudo-su-coffee/porter/issues" target="_blank" rel="noreferrer" className="plan-link">Start a conversation <ArrowUpRight size={15} /></a></article>
          </div>
        </section>

        <section className="quote-section section-wrap">
          <div className="quote-mark">“</div>
          <blockquote>Build the platform you can explain on a whiteboard.</blockquote>
          <div className="quote-meta"><span className="quote-line" />Porter is for teams that want the useful parts of a PaaS—and the final say over the host.</div>
        </section>

        <section id="get-started" className="cta-section section-wrap">
          <div className="cta-card">
            <div><div className="eyebrow eyebrow--light"><span className="eyebrow-rule" />Your next step is small</div><h2>Put one app<br /><em>on one host.</em></h2></div>
            <div className="cta-side"><p>Follow the first-deploy guide, validate the control plane, and learn the path before you add another layer.</p><a className="button button--light" href="/getting-started#install">Start the first deploy <ArrowRight size={17} /></a></div>
          </div>
        </section>
      </main>

      <footer className="site-footer section-wrap">
        <a className="brand-lockup" href="#top"><PorterMark /><span>porter</span></a>
        <span>self-hosted deployment, without the ceremony</span>
        <div className="footer-links"><a href="https://github.com/sudo-su-coffee/porter" target="_blank" rel="noreferrer">GitHub <ArrowUpRight size={14} /></a><a href="#top">Back to top <ArrowRight size={14} /></a></div>
      </footer>
    </div>
  );
}
