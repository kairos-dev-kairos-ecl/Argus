# ARGUS XDR Platform - Onum/Falcon Style Refactor

## Overview
Complete refactoring of the ARGUS XDR Platform with Onum/Falcon-inspired aesthetic, featuring advanced telemetry visualizations, rich entity tracing, and interactive correlation capabilities.

## Key Enhancements

### 1. **Enhanced Trace Flow** (`/trace`)
Rich entity tracing with React Flow-based pipeline visualization.

**Features:**
- **Node-based Pipeline Builder**: Interactive flow diagram showing end-to-end trace execution
- **Prompt Lifecycle Mapping**: Dedicated nodes for parsing → sanitization → tokenization → inference
- **Entity Encapsulation**: Each node contains "What, How, Where, When" metadata
- **Interactive Command Palette** (⌘K / Ctrl+K):
  - Global correlation search
  - Real-time node and edge highlighting
  - Dims non-relevant network segments
- **Right-side Details Panel**: Comprehensive entity information on node selection

**Visual Style:**
- Rounded corners (8px radius) for modern aesthetic
- Node-specific color coding (purple gradients for prompt phases, cyan for inference)
- Glowing selection effects
- Animated edges showing data flow

### 2. **Enhanced Telemetry Dashboard** (`/telemetry`)
Onum/Falcon-style data flow visualization with Sankey diagrams and smooth area charts.

**Features:**
- **Macro vs. Micro Scope Toggle**:
  - **Org-Wide**: Aggregated Sankey flows showing data volumes across collectors, pipelines, and sinks
  - **User Journey**: Individual user trace execution paths and metrics
- **Sankey Diagrams** (ECharts):
  - Glowing gradient flows (blue → purple → pink → amber)
  - Interactive tooltips with volume metrics
- **Time-Series Charts**:
  - Smooth area-fill line charts for EPS, latency, throughput
  - High-contrast data lines with gradient fills
  - Hover-state metric tooltips
- **Real-time Metrics**:
  - Live event counters with color-coded statuses
  - Recent trace history with latency indicators

**Visual Style:**
- Dark mode base (#050506 background)
- Neon gradients (cyan #00F0FF, purple #8B5CF6, amber #FBBF24, pink #EC4899)
- Glowing card effects with colored borders
- Ultra-dense data presentation

### 3. **Enhanced Hunting Console** (`/hunt`)
Advanced query interface with analytics panel and latency distribution visualizations.

**Features:**
- **Query Editor**: Full-featured SQL-like query interface
- **Quick Query Snippets**: Pre-built queries for common hunting scenarios
- **Real-time Results**: Expandable result cards with JSON payload inspection
- **Right Analytics Panel**:
  - Latency distribution bar chart
  - Success rate metrics
  - Top models breakdown
  - Error rate monitoring
- **Status Indicators**: Color-coded result cards (success/warning/error)

**Visual Style:**
- Split-panel layout (editor + results + analytics)
- Glowing borders on result cards matching status
- ECharts bar charts with gradient colors
- Syntax-highlighted JSON payloads

## Technical Stack

### New Dependencies
- `@xyflow/react`: React Flow for node-based visualizations
- `echarts`: ECharts for Sankey diagrams and advanced charts
- `echarts-for-react`: React wrapper for ECharts
- `cmdk`: Command palette for correlation search

### Design Tokens
```css
/* New Gradient Colors */
--color-blue-light: #3B82F6
--color-purple: #8B5CF6
--color-purple-light: #A78BFA
--color-pink: #EC4899
--color-amber: #FBBF24
--color-teal: #14B8A6
--color-green: #10B981

/* Soft Radius for Modern Aesthetic */
--radius-soft: 8px
```

## Navigation Structure

```
ARGUSXDR Platform
├── ONBOARDING      → Multi-step validation flow
├── ENTITY TRACE    → React Flow pipeline (NEW)
├── TELEMETRY       → Sankey + time-series (NEW)
├── HUNT            → Enhanced analytics console (NEW)
└── AUDIT           → Immutable ledger
```

## Key Interactions

### Correlation Search (⌘K)
1. Press `⌘K` (Mac) or `Ctrl+K` (Windows/Linux)
2. Type entity, IP, model, or any searchable string
3. System highlights matching nodes and connecting edges
4. Click on match to view detailed entity information
5. Non-relevant paths are dimmed for focus

### Scope Toggle (Telemetry Dashboard)
1. Click "ORG-WIDE" to view macro-level Sankey data flows
2. Click "USER" to zoom into specific user journey
3. Select user from dropdown to view individual traces
4. Charts update to show user-specific metrics

### Query Hunting
1. Use Quick Query snippets for common patterns
2. Edit query in SQL-like syntax
3. Execute to see results with status indicators
4. Expand cards to inspect full JSON payloads
5. Monitor analytics panel for distribution insights

## Visual Aesthetic Comparison

### Before (Brutalist)
- Pure black background (#050506)
- Zero border radius
- Harsh cyan accents
- Dense tables and lists
- Minimal visual hierarchy

### After (Onum/Falcon Enhanced)
- Dark mode base with depth (#050506 → #111216)
- Soft rounded corners (8px) on modern elements
- Rich gradient flows (blue → purple → pink)
- Sankey diagrams and flow visualizations
- Glowing effects and animated connections
- Clear visual hierarchy with color coding

## Performance Optimizations
- React Flow optimized for large graph rendering
- ECharts uses canvas for smooth 60fps animations
- Memoized chart options to prevent re-renders
- Virtualized result lists for scalability

## Browser Support
- Modern browsers with CSS Grid/Flexbox
- Canvas 2D support for ECharts
- ES6+ JavaScript features
- Keyboard shortcuts (⌘K, Ctrl+K)

## Future Enhancements
- Real-time WebSocket streaming for live telemetry
- Export functionality for Sankey diagrams
- Custom query builder with visual interface
- Advanced correlation algorithms
- Multi-tenant filtering and isolation
- Time-range scrubbing for historical analysis
