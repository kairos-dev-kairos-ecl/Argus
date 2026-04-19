
# ArgusXDR Tactical Hub

## Product Overview

**The Pitch:** A unified, ultra-dense observability engine built for real-time AI/ML telemetry and agentic tracing. It prioritizes deep LLM inference tracking, GPU hardware utilization, and complex RAG tool chains to collapse the MTTR (Mean Time To Resolution) for generative AI system bottlenecks and token generation anomalies.

**For:** AI/ML Researchers, MLOps Engineers, and System Architects who need zero-latency, high-fidelity AI telemetry correlation without UI fluff.

**Device:** desktop

**Design Direction:** Brutalist, high-contrast tactical dark mode. Aggressive neon accents against abyssal black, prioritizing ultra-dense data rendering and razor-sharp borders.

**Inspired by:** Datadog, CrowdStrike Falcon

---

## Observability Layers and Coverage

The platform explicitly categorizes telemetry into ten critical AI/ML strata:

- **L1 Hardware/GPU:** CUDA utilization, VRAM allocation, thermal throttling, and multi-node interconnects.
- **L2 Network/Interconnect:** NVLink topology, InfiniBand throughput, and cross-cluster packet latency.
- **L3 Infrastructure/Orchestration:** Container orchestration, node scheduling, and resource allocation overhead.
- **L4 Data Pipeline:** Ingestion streams, batch processing queues, and raw data throughput.
- **L5 Model/Inference:** Token throughput, TTFT (Time To First Token), generation latency, and KV cache metrics.
- **L6 Memory/Vector Storage:** Vector search latency, embedding generation overhead, and indexing times.
- **L7 Agentic/RAG:** Tool execution chains, multi-agent orchestration, and retrieval context payload monitoring.
- **L8 Guardrails/Security:** Prompt injection detection, PII redaction latency, and semantic filtering overhead.
- **L9 Logic/API:** API gateway routing, internal endpoint health, and overarching service request latency.
- **L10 Application/User:** End-user facing request latency, interaction success rates, and total session execution time.

---

## Screens

- **XDR Onboarding & Validation Flow:** Multi-step, resumable wizard for Org setup, Ingestion Method Selection, API Token vaulting, and live Pre-Dashboard connection validation.
- **Traceability & Zero-Trust Audit Ledger:** Ultra-dense, immutable data grid tracking all system actions, token usage events, and setup states to enforce complete zero-trust accountability.
- **AI System Health & Layers Dashboard:** Overview of the 10 observability layers, featuring an infinite-scroll telemetry stream and the L1-L10 Horizon Grid for real-time aggregate model performance metrics.
- **Agent & Inference Trace Waterfall:** Split-pane execution timeline mapping LLM span tracing, token counts, and raw prompt/response payloads for immediate bottleneck observability.
- **AI Telemetry & Hunting Console:** Monospace query builder and raw payload/log inspection pane for hunting specific model versions, token usage anomalies, and latency spikes.

---

## Key Flows

**Trace Isolation & Root Cause Drill-Down:** Isolate a critical latency anomaly to its origin inference trace or hardware bottleneck.

1. User is on AI System Health & Layers Dashboard -> sees `CRITICAL: L5_TTFT_DEGRADATION` glowing in neon red within the L1-L10 Horizon Grid.
2. User clicks alert row -> splits screen, locking the Agent & Inference Trace Waterfall in the right pane.
3. Trace waterfall highlights the exact bottlenecked LLM span or overloaded GPU node -> user inspects the raw prompt payload causing the latency.

---

<details>
<summary>Design System</summary>

## Color Palette

- **Primary:** `#00F0FF` - Neon Cyan for interactive elements and active focus
- **Background:** `#050506` - Abyssal black
- **Surface:** `#111216` - Panels, raw data blocks
- **Text:** `#E9ECEF` - Stark off-white for legibility
- **Muted:** `#343A40` - Borders, inactive states, gridlines
- **Accent:** `#FF2A00` - Critical alerts, fatal errors
- **Warning:** `#FFB300` - Amber for warnings, degraded state

## Typography

- **Headings:** `Space Grotesk`, 700, 20-28px
- **Body:** `JetBrains Mono`, 400, 13px
- **Small text:** `JetBrains Mono`, 400, 11px
- **Buttons:** `Space Grotesk`, 600, 12px, uppercase, 1px tracking

**Style notes:** Brutalist aesthetic. `0px` border radius everywhere. Sharp 1px `#343A40` borders. Drop shadows are replaced with solid 2px offset blocks or aggressive `0 0 12px` neon text glows for critical alerts.

## Design Tokens

```css
:root {
  --color-primary: #00F0FF;
  --color-background: #000000;
  --color-surface: #111216;
  --color-text: #E9ECEF;
  --font-primary: 'JetBrains Mono', monospace;
  --font-display: 'Space Grotesk', sans-serif;
  --radius: 0px;
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --border-stark: 1px solid #343A40;
}
```

## Navigation model: 

- **Tab bar / Left sidebar: linking all 5 screens.
- **Syntax highlighting: Use Prism.js (CDN-safe) or specify manual, <span> coloring approach.

## Trace waterfall: 

- **Clarify — CSS flexbox Gantt bars only (no canvas).

## Tailwind strategy: 
- **Use CSS custom properties + inline styles for all non-standard colors. Drop arbitrary value syntax.

## Animations: 
- **Either drop matrix/glitch effects or spec them explicitly (e.g., "text scramble: cycle through random chars at 50ms interval for 800ms").

</details>

---

<details>
<summary>Screen Specifications</summary>

### XDR Onboarding & Validation Flow

**Purpose:** Secure, resumable setup of Org context, ingestion pipelines, and API token generation with real-time pre-dashboard connection validation.

**Layout:** Centered terminal-style wizard panel (max-width 800px) against the abyssal black background. 30/70 vertical split inside the panel: Progress tracker on the left, active step content on the right.

**Key Elements:**
- **Step Tracker:** Vertical timeline tracking Org Setup, Ingestion Method, Token Vaulting, and Validation.
- **Token Vault Component:** Obfuscated input field with a stark copy-to-clipboard button and a one-time viewing warning rendered in Amber (`#FFB300`).
- **Validation Console:** Embedded mini-terminal showing live connection handshakes and initial L1-L4 telemetry pings to confirm successful ingestion.

**States:**
- **Empty:** Blinking block cursor awaiting initial Org Name input.
- **Loading:** Matrix-style text scrambling during API token generation and handshake validation.
- **Error:** Neon red border flash (`#FF2A00`) and explicit rejection reason (e.g., `ERR_AUTH_FAILED`).

**Components:**
- **Step Indicator:** `24x24px`, solid `bg-[#111216]`, `1px` border, text `white`, active state `bg-[#00F0FF] text-[#050506]`.

**Interactions:**
- **Click Next:** Validates current step data via zero-trust API endpoints before advancing.
- **Copy Token:** Triggers a 2-second `#00F0FF` highlight flash on the vault input field.

**Responsive:**
- **Desktop:** Full centered wizard panel.
- **Tablet:** Desktop only.
- **Mobile:** Desktop only.

### Traceability & Zero-Trust Audit Ledger

**Purpose:** Immutable, high-density logging of all system actions, token consumption, and architectural modifications to enforce strict zero-trust principles.

**Layout:** Full-width grid structure. Fixed header containing deep search/filter inputs, and a sticky bottom pagination/status bar.

**Key Elements:**
- **Immutable Grid:** Monospace data table, 20px row height. Columns: Timestamp (UTC), Actor/Agent, Action, Target Layer (L1-L10), Diff/Payload, Hash.
- **Zero-Trust Filters:** Granular dropdowns to filter by IAM role, IP origin, or specific AI/ML stratum.
- **Cryptographic Hash:** A column displaying the shortened SHA-256 hash of the audit event, visually confirming immutability.

**States:**
- **Empty:** `NO AUDIT EVENTS FOUND` centered in grid.
- **Loading:** Top progress bar, 1px `#00F0FF` scanning left to right.
- **Error:** Row background `#300000` for tampered or structurally invalid log entries.

**Components:**
- **Hash Badge:** `font-family: JetBrains Mono`, 10px, `#E9ECEF` text on `#111216` background, sharp 1px `#343A40` border.

**Interactions:**
- **Click Row:** Expands an inline accordion showing the full JSON diff of the action (before/after states).
- **Hover Hash:** Shows full cryptographic signature tooltip.

**Responsive:**
- **Desktop:** Full grid rendering.
- **Tablet:** Columns collapse to most critical (Timestamp, Actor, Action, Hash).
- **Mobile:** Desktop only requirement.

### Agent & Inference Trace Waterfall

**Purpose:** Visualizing LLM chain execution, agentic tool usage, and generation latency over time.

**Layout:** 50/50 horizontal split. Top pane: Canvas payload viewer. Bottom pane: Gantt-style trace waterfall.

**Key Elements:**
- **Payload Viewer:** Dark canvas displaying raw prompt and response payloads, highlighted token counts, and tool execution outputs.
- **Trace Waterfall:** 20px height bars representing LLM spans, vector searches, and API calls. Labeled with model names and spanning method names inside the bar, `JetBrains Mono 10px`.
- **Span Detail Overlay:** 200px wide floating HUD triggered on span hover, showing TTFT, total tokens, generation latency, and associated GPU memory.

**States:**
- **Empty:** `NO INFERENCE CONTEXT FOUND`.
- **Loading:** Skeleton bars wiping left to right.
- **Error:** Solid red error boundary, retry button.

**Components:**
- **Span Bar:** `height: 16px`, `bg-[#00F0FF]`, `opacity: 0.8`, `border-left: 2px solid white`.

**Interactions:**
- **Click Span:** Loads exact prompt and response payload into the top payload viewer.
- **Hover Span:** Displays exact millisecond latency tooltip and token metadata.

**Responsive:**
- **Desktop:** 50/50 horizontal split.
- **Tablet:** Waterfall becomes full screen, Payload Viewer hidden behind toggle.
- **Mobile:** Desktop only.

### AI System Health & Layers Dashboard

**Purpose:** Rapid ingestion and categorization of global AI telemetry across the 10 observability layers (L1-L10).

**Layout:** Persistent 48px global time-scrubber top nav. Left sidebar (240px) for model and environment filters. Main area split 70/30: Infinite-scroll telemetry stream (left) and L1-L10 Horizon Grid (right).

**Key Elements:**
- **Global Time-Scrubber:** Fixed top, `11px JetBrains Mono`, absolute UTC time bounds, interactive histogram.
- **Telemetry Stream:** Ultra-dense table, 24px row height, no padding, alternating `#050506` and `#0A0A0C` rows. Shows request logs, token usage, and errors.
- **L1-L10 Horizon Grid:** 10 micro-charts rendering anomaly scores from Hardware (L1) up to the Application (L10). High-contrast metric blocks utilizing `FF2A00` for active critical latency spikes or CUDA Out-Of-Memory errors.

**States:**
- **Empty:** `AWAITING TELEMETRY` in dim gray.
- **Loading:** Pulsing cyan 1px scanline effect.
- **Error:** Glitch effect text, `ERR_STREAM_DEAD`.

**Components:**
- **Alert Badge:** `60x20px`, `1px` border, inset text, no radius, `bg-transparent text-[#FF2A00] border-[#FF2A00]`.

**Interactions:**
- **Click Alert Row:** Triggers slide-out detail pane (right side) to inspect raw model logs/metrics.
- **Hover Row:** Row background shifts to `#1A1D24`, cursor `crosshair`.

**Responsive:**
- **Desktop:** Full 4-pane view.
- **Tablet:** Horizon Grid drops below telemetry stream.
- **Mobile:** Hidden. Desktop only requirement.

### AI Telemetry & Hunting Console

**Purpose:** Proactive investigation via raw telemetry, log, and payload query execution to analyze model versions, token usage, and latency.

**Layout:** 30/70 vertical split. Top: Multi-line query editor. Bottom: Raw JSON payload/log results.

**Key Elements:**
- **Query Editor:** Monospace textarea, syntax highlighting (`Cyan` operators, `Amber` strings). Designed for querying specific LLM versions, token constraints, or TTFT ranges.
- **Run Button:** 120x40px, sharp corners, `#00F0FF` border, text `EXECUTE`.
- **Payload Pane:** Expandable/collapsible JSON tree, strict indentation, muted gray keys, white values. Focuses on deeply nested agentic logs and RAG traces.

**States:**
- **Empty:** Blinking block cursor in editor.
- **Loading:** Spinning ASCII `| / - \` in run button.
- **Error:** Payload pane turns deep red `#300000`, dumps stack trace.

**Components:**
- **Query Snippet Chip:** `padding: 2px 6px`, `bg-[#111216]`, `border: 1px solid #343A40`, inserts text to editor on click.

**Interactions:**
- **Click Run:** Triggers loading state, locks editor, paints results down.
- **Hover JSON Key:** Underlines key, shows absolute JSON path in footer.

**Responsive:**
- **Desktop:** Split view.
- **Tablet:** Editor shrinks, Payload pane takes over.
- **Mobile:** Desktop only.

</details>

---

<details>
<summary>Build Guide</summary>

**Stack:** HTML + Tailwind CSS v3

**Build Order:**
1. **XDR Onboarding & Validation Flow** - Sets up the zero-trust Org foundation, token generation, and the brutalist layout logic for wizard forms.
2. **Agent & Inference Trace Waterfall** - Introduces complex custom canvas/CSS visualizations and split-pane layout logic, establishing the core AI observability requirement.
3. **AI System Health & Layers Dashboard** - Establishes the core brutalist typography, ultra-dense data tables, and the critical global navigation framework across the 10 observability layers.
4. **AI Telemetry & Hunting Console** - Defines input fields, syntax highlighting, and raw data display components crucial for querying model versions and token logs.
5. **Traceability & Zero-Trust Audit Ledger** - Leverages the dense table patterns from the Dashboard but introduces complex filtering UI and immutable data rendering states to enforce zero-trust policies.

</details>

