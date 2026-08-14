# Porter Marketing Site — Visual Direction

## Three possible directions

### Theme Name: Harbor Glass
**Very Brief Intro:** A calm, tactile infrastructure aesthetic using sea-glass translucency, warm paper tones, and precise graphite typography. It makes Porter feel trustworthy and technical without looking like a developer-tool cliché.
**Probability:** 0.07

### Theme Name: Field Manual
**Very Brief Intro:** An editorial operations look inspired by printed field guides, incident notebooks, and annotated system diagrams. It would feel practical, durable, and slightly utilitarian.
**Probability:** 0.03

### Theme Name: Signal House
**Very Brief Intro:** A darker, high-contrast control-room direction with restrained amber status signals and strong typographic blocks. It would feel more dramatic and command-oriented while remaining professional.
**Probability:** 0.09

## Selected approach: Harbor Glass

### Design Movement

Contemporary editorial product design with material glass, Swiss information design, and a restrained coastal palette. The glass effect is treated as a physical surface over a warm architectural background, not as a default blur filter.

### Core Principles

1. **Calm precision:** technical ideas are explained with clear hierarchy, short copy, and deliberate whitespace.
2. **Material depth, not decoration:** translucency, hairline borders, and soft shadows clarify grouping and hierarchy.
3. **Asymmetric confidence:** layouts use offset content, side annotations, and anchored diagrams instead of repeated centered hero blocks.
4. **Operational proof:** the page shows how Porter works through a visual deploy flow rather than relying on vague superlatives.

### Color Philosophy

The base is a warm mineral white rather than sterile blue-white. Graphite provides authority and readability. Deep sea-glass teal is the signature color for trust, routing, and active states. A muted apricot signal is reserved for “live” or “deploying” moments so status color feels intentional. There are no purple gradients and no neon glow language.

### Layout Paradigm

The page moves from a narrow, left-aligned navigation into an offset hero with the copy on the left and a floating system card on the right. Subsequent sections alternate between a broad editorial statement and a contained technical vignette. The deploy flow is presented as a horizontal rail on desktop and a vertical sequence on mobile. Content is anchored to a visible vertical rhythm rather than centered in identical cards.

### Signature Elements

1. **Glass instrument panels:** translucent cards with one strong highlight edge, a quiet inner shadow, and a small physical “handle” or status chip.
2. **Route line motif:** a thin teal line with node markers travels through the hero and deployment flow, echoing networking without becoming a generic circuit pattern.
3. **Porter mark:** a compact four-gate symbol suggesting a port, a gateway, and a microVM boundary; no wordmark text inside the mark.

### Interaction Philosophy

Interactions should feel like touching a well-made instrument. Buttons respond with a small pressure shift, cards lift by a few pixels, and navigation links reveal a measured underline. Motion is quick and quiet. The primary CTA scrolls to the deployment flow; secondary actions open the relevant external link or show a concise “coming soon” toast only where a real destination is not available.

### Animation

Use only opacity and transform for motion. Reveal the hero card with a 40–60ms stagger and a short upward drift. Let the route line gently draw in once on first load, then remain still. Hover states should settle within 180ms. Respect `prefers-reduced-motion` by disabling the entrance drift and line draw while preserving focus and hover affordances.

### Typography System

Use **Manrope** for display and navigation labels, with **IBM Plex Mono** for technical metadata, status labels, and code-like details. Headlines are compact, high-weight, and slightly tight. Body copy uses a relaxed line height and a maximum width of 58ch. Technical labels are uppercase or monospace only when they carry system meaning.

### Brand Essence

Porter is the self-hosted deployment layer for teams that want microVM isolation without the operational weight of a cluster. Personality: **assured, tactile, exacting**.

### Brand Voice

Headlines should be direct and composed. CTAs should name the next action instead of using generic conversion language. Microcopy should sound like an experienced operator explaining a system to another experienced operator.

Example headline: “Your own deployment plane, on one box.”

Example CTA: “Trace a deploy”

### Wordmark & Logo

The mark is a geometric four-gate symbol: two short vertical apertures connected by a horizontal bridge, with a small offset notch implying a route entering and leaving an isolated boundary. It should be rendered as a bold graphic symbol, never as the brand name in a default font. The wordmark uses a custom-feeling Manrope lockup with a slightly extended crossbar on the “t” in Porter.

### Signature Brand Color

**Porter Sea Glass — `#2C8C88`**. It is deep enough for readable text and controls, soft enough to sit naturally against warm paper, and distinct from the conventional electric blue of cloud tooling.

## Style Decisions

- Keep glass surfaces limited to the hero instrument, feature callouts, and one deployment panel; large page areas stay matte so the material reads as intentional.
- Keep the palette warm, sea-glass, and graphite. Do not introduce purple gradients, pure black backgrounds, or cyan neon.
- Use a route line and node language to make the platform architecture visually legible.
- Prefer short, specific copy over generic SaaS filler.
- Carry a thin sea-glass route line with node markers across homepage and documentation as Porter’s primary wayfinding language.
- Keep ordinary cards matte and quiet; reserve glass treatment for key instrument surfaces with status, metadata, or depth cues.
- Make every visual vignette explain an operational state such as source, boundary, route, replica, health, or deploy status.
- Use short, human-first product statements with one concrete promise per section; let the interface and operational proof carry the technical depth.
- The voice should feel calm, confident, and exact: never hype, never jargon for its own sake, and never more words than the visitor needs to take the next step.
