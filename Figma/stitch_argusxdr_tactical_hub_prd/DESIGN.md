# Design System Documentation: High-Density Data Architecture

## 1. Overview & Creative North Star
### Creative North Star: "The Kinetic Observatory"
This design system is engineered to transform raw, high-volume telemetry into an immersive, editorial experience. We are moving away from the "SaaS-standard" look of isolated cards and heavy containers. Instead, the UI acts as a lens—a "Kinetic Observatory"—where data density is celebrated rather than hidden. 

The aesthetic leverages **intentional asymmetry** and **tonal depth** to guide the eye through complex agent topologies. We replace rigid structural boundaries with fluid, glowing connections and multi-layered glass surfaces, creating an environment that feels alive, precise, and authoritative.

---

## 2. Colors & Surface Logic

### The Kinetic Palette
Our color strategy utilizes a deep neutral core (`background: #0e0e12`) to allow high-vibrancy accent gradients to signify data movement.

- **Primary & Secondary Flow:** Use `primary` (#8ff5ff) to `secondary` (#d575ff) gradients for standard data pipelines.
- **Tertiary Heat:** Use `tertiary` (#ff51fa) to amber-tinted gradients for high-priority or high-volume data streams.
- **The "No-Line" Rule:** Sectioning must never be achieved with 1px solid solid borders. Boundaries are defined by transitions between `surface-container-lowest` (#000000) and `surface-container-low` (#131317). 

### Surface Hierarchy & Nesting
Treat the interface as a physical stack of semi-transparent materials:
1.  **Base Layer:** `surface` (#0e0e12) - The canvas.
2.  **Injected Content:** `surface-container-low` (#131317) - Used for primary workspace regions.
3.  **Floating Nodes:** `surface-container-high` (#1f1f25) - For interactive cards or data modules.
4.  **Overlays:** `surface-bright` (#2c2b32) with 40% opacity and 24px backdrop-blur.

**Signature Texture:** Apply a 1px inner glow using `on-surface` (#fcf8fe) at 10% opacity to the top edge of all glass containers to simulate light catching on a physical edge.

---

## 3. Typography
We utilize a dual-font system to balance editorial personality with technical precision.

| Level | Font Family | Size | Character | Use Case |
| :--- | :--- | :--- | :--- | :--- |
| **Display** | Space Grotesk | 2.25rem - 3.5rem | Bold, tight tracking | Hero metrics, section entries |
| **Headline** | Space Grotesk | 1.5rem - 2rem | Regular / Medium | Page titles, major module headers |
| **Title** | Inter | 1rem - 1.375rem | Semi-bold | Card titles, group labels |
| **Body** | Inter | 0.875rem | Regular | Descriptions, tooltips |
| **Data (Mono)** | JetBrains Mono | 10px - 11px | Monospaced | High-density tables, logs, IDs |

*Note: For the high-density tables requested, JetBrains Mono at 10px is the mandatory standard to ensure columns align perfectly across vertical flows.*

---

## 4. Elevation & Depth

### The Layering Principle
Depth is achieved through **Tonal Stacking**. A card does not sit "on" a background; it is a "higher tier" of the background.
- Place `surface-container-highest` cards on a `surface-dim` background.
- **The Ghost Border:** If a separator is required for accessibility, use the `outline-variant` (#48474c) at 15% opacity. Never use 100% opaque lines.

### Ambient Shadows
Shadows must be "atmospheric." 
- **Token:** `0px 12px 32px rgba(0, 0, 0, 0.4)`. 
- For active "Floating" nodes, use a tinted shadow: `0px 8px 24px rgba(143, 245, 255, 0.08)` (using a ghost of the `primary` token).

---

## 5. Components

### High-Density Data Tables
- **Padding:** 4px vertical, 8px horizontal.
- **Styling:** No row dividers. Use a subtle `surface-container-highest` hover state for the entire row.
- **Type:** 10px JetBrains Mono for all cell content.

### Sankey Agent Topology
- **Lines:** 1px width connecting lines using `primary-dim` or `secondary-dim`.
- **Flow Effect:** Apply a `linear-gradient` that moves from the source node color to the destination node color.
- **Glow:** Add a 2px `box-shadow` blur to lines representing active "Live" data streams.

### Interactive Buttons
- **Primary:** Gradient fill (`primary` to `primary-container`). 1px inner glow.
- **Secondary:** Transparent background with a `Ghost Border` (15% opacity `on-surface`).
- **Rounding:** Use `sm` (0.125rem) for a sharp, technical look.

### Navigation Sidebar
- Forbid dividers. Use `spacing-lg` (24px) gaps between icon groups.
- Active state is indicated by a `surface-container-high` background shift and a 2px vertical `primary` line on the left-most edge.

---

## 6. Do's and Don'ts

### Do
- **DO** embrace high density. If a screen feels "empty," increase the data granularity in the Mono typeface.
- **DO** use backdrop-blur on all modals and popovers to maintain the "frosted glass" aesthetic.
- **DO** use 4px/8px spacing increments for all internal component padding to maintain the ultra-tight technical grid.
- **DO** use vibrant multi-colored gradients for Sankey flows to indicate different data types (e.g., Cyan to Purple for metadata, Magenta to Amber for security events).

### Don't
- **DON'T** use 1px solid black or high-contrast white borders to separate sections.
- **DON'T** use standard rounded corners. Stick to the `sm` (0.125rem) or `none` (0px) tokens for a more professional, "instrument-panel" feel.
- **DON'T** use shadows on flat, non-interactive elements. Reserve elevation for true "floating" states.
- **DON'T** use `Inter` for data. `JetBrains Mono` is the only permissible font for numerical values and technical logs.