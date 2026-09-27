## Overview

Voz do Povo turns dense Brazilian government open data into calm, readable reports about the politicians people elected. The design voice is **civic editorial**: white canvas, dark ink type, generous whitespace, and data — big numbers, bars, tables — as the visual hero. Nothing decorative competes with the numbers.

The palette descends from the Brazilian flag (green, yellow) extended with a deep purple second series and a red reserved strictly for warnings. Brand voltage doesn't come from gradient washes; it comes from **full-bleed signature bands** in `{colors.signature-green}` and `{colors.signature-purple}` that punctuate long-scroll report pages, and from the confidence of a `{colors.accent}` green CTA sitting in clean whitespace.

Density is the enemy. Every screen answers one question at a time: a headline number, a short plain-language explanation, then breathing room. Tooltips and explainer boxes carry the civic-education load so the main flow stays light.

**Key Characteristics:**

- Primary CTA is `{colors.accent}` (green) with white text and a `{rounded.md}` corner — the color of the flag's green field, used sparingly so it always means "act here."
- Yellow is a **marker**, not a background wash: permanent type badges (Emenda PIX), highlight rules, small flags. It never fills a hero.
- Red exists **only** as a warning signal (above-median expense badges, attention rollups). Using red decoratively is off-brand — it would cry wolf.
- Purple is the second data series: comparison bars, secondary charts, the house-toggle accent for Senado vs Câmara.
- Hero is white canvas. No gradient, no mesh, no aurora. Trust comes from the search bar, the type, and the whitespace.
- Section rhythm on report pages: white canvas → stat cards → green explainer → white body → dark signature band → footer. The canvas resets between signature surfaces.
- Border radius is friendly but disciplined: `{rounded.md}` (10px) cards, `{rounded.pill}` for the search input and toggle pills only.
- Vertical rhythm is `{spacing.section}` (64px) between major bands — tighter than a marketing site because reports are dense.

## Colors

### Palette (raw scales — hexes live here only)

From the design-system color tables, locked in `app/globals.css` layer 1.

- **Green ramp** (flag green, primary): `{colors.green-900}` #00541D · `{colors.green-800}` #007B2A · `{colors.green-700}` #009D35 · `{colors.green-500}` #15BF4E · `{colors.green-300}` #39CC6B
- **Purple ramp** (secondary #1, second data series): `{colors.purple-900}` #19064F · `{colors.purple-800}` #270B73 · `{colors.purple-700}` #331094 · `{colors.purple-500}` #4B25B4 · `{colors.purple-300}` #6746C4
- **Yellow ramp** (flag yellow, marker): `{colors.yellow-900}` #736100 · `{colors.yellow-800}` #A78C00 · `{colors.yellow-700}` #D7B400 · `{colors.yellow-500}` #FFDA1C · `{colors.yellow-300}` #FFE147
- **Red ramp** (complement, warning only): `{colors.red-900}` #731200 · `{colors.red-800}` #A71A00 · `{colors.red-700}` #D72100 · `{colors.red-500}` #FF3F1C · `{colors.red-300}` #FF6347
- **Neutral ramp** (text + surfaces, own scale): `{colors.neutral-900}` #1A2332 · `{colors.neutral-600}` #5B6779 · `{colors.neutral-300}` #DDE3EC · `{colors.neutral-100}` #F7F8FA · `{colors.white}` #FFFFFF

### Semantic Tokens (components reference ONLY these)

**Text & Surfaces**

- **Ink** (`{colors.ink}` → neutral-900): strongest text — headlines, big numbers.
- **Muted** (`{colors.muted}` → neutral-600): captions, source lines, table headers, secondary copy.
- **Line** (`{colors.line}` → neutral-300): 1px hairlines — card borders, table dividers, pill outlines.
- **Background** (`{colors.bg}` → neutral-100): page floor.
- **Card** (`{colors.card}` → white): card and input surfaces.

**Brand Accent (green)**

- **Accent** (`{colors.accent}` → green-800): primary CTA bg, active pills, ranking bars, focus rings.
- **Accent Strong** (`{colors.accent-strong}` → green-700): the pure flag green — brand marks, icons.
- **Accent Hover** (`{colors.accent-hover}` → green-900): press/hover on accent.
- **Accent Soft** (`{colors.accent-soft}` → green-300): trend bars, tints (used via `color-mix` at ~30% for fills).

**Second Series (purple)**

- **Accent 2** (`{colors.accent-2}` → purple-700): second chart series, Senado-variant accents.
- **Accent 2 Soft** (`{colors.accent-2-soft}` → purple-300): purple tints and fills.

**Warning (red — signal only)**

- **Warn** (`{colors.warn}` → red-800): attention badge text, warning links.
- **Warn Strong** (`{colors.warn-strong}` → red-700): emphasis inside warnings.
- **Warn Soft** (`{colors.warn-soft}` → red-300): badge fill base (via `color-mix` at ~35% over white).

**Marker (yellow)**

- **Marker** (`{colors.marker}` → yellow-900): text on yellow badges.
- **Marker BG** (`{colors.marker-bg}` → yellow-300): badge fill (Emenda PIX).
- **Marker Line** (`{colors.marker-line}` → yellow-700): badge outline.

### Signature Surfaces

Brand voltage carriers — full-bleed bands and large cards, never small accents:

- **Signature Green** (`{colors.signature-green}` → green-900): deep-green report band with white type — the "explain the money" moment on politician pages.
- **Signature Purple** (`{colors.signature-purple}` → purple-900): dark purple band for cross-chamber comparisons and methodology callouts.
- **Signature Dark** (`{colors.signature-dark}` → neutral-900): near-ink CTA/footer band.
- **Signature Cream-Yellow** (`{colors.signature-cream}` → yellow-300 at ~25% via color-mix): soft callout band for glossary/education moments — flag yellow whispered, not shouted.

## Typography

### Font Family

System stack (`system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`) — zero font-download latency, native feel on every device. If a display face is ever added, **Inter** (variable) is the house substitute. Data is the hero, so `font-variant-numeric: tabular-nums` is mandatory on every numeric display.

### Hierarchy

| Token                      | Size | Weight | Line Height | Use                                          |
| -------------------------- | ---- | ------ | ----------- | -------------------------------------------- |
| `{typography.display-lg}`  | 34px | 700    | 1.2         | Home h1, big-number stat                     |
| `{typography.display-md}`  | 26px | 700    | 1.25        | Stat-trio values (empenhado / pago / % pago) |
| `{typography.title-md}`    | 20px | 700    | 1.35        | Card titles, politician name                 |
| `{typography.label-caps}`  | 16px | 700    | 1.4         | Uppercase card headers (0.04em tracking)     |
| `{typography.body}`        | 16px | 400    | 1.55        | Running text, explainer boxes                |
| `{typography.body-sm}`     | 14px | 400    | 1.5         | Table cells, secondary copy                  |
| `{typography.caption}`     | 13px | 400    | 1.4         | Sub-lines, trend labels                      |
| `{typography.meta}`        | 12px | 400    | 1.35        | Source lines ("Fonte: …"), item subtext      |
| `{typography.badge}`       | 12px | 600    | 1.2         | Emenda PIX marker badge                      |

### Principles

Big numbers carry weight 700 — they are the product. Everything else stays at 400/600 so the numbers stay loudest. Portuguese copy is written for a citizen, not a lawyer: short sentences, no jargon without an inline `{component.tooltip}` or `{component.explainer}` definition. Letter-spacing stays at 0 except uppercase labels.

## Layout

### Spacing System

- **Base unit:** 4px (all spacing snaps to 4-multiples).
- **Tokens:** `{spacing.xs}` 4px · `{spacing.sm}` 8px · `{spacing.md}` 12px · `{spacing.lg}` 16px · `{spacing.xl}` 24px · `{spacing.xxl}` 32px · `{spacing.section}` 64px.
- **Card internal padding:** `{spacing.xl}` (20px on data cards — density matters inside a card, whitespace lives between them).
- **Signature band padding:** `{spacing.section}` vertical, white type centered or left-rail.
- **Gutters:** `{spacing.lg}` (16px) between cards in ranking grids.

### Grid & Container

- **Max content width:** 860px centered (`{layout.wrap}`) — a reading column, not a dashboard wall. Dense data earns focus, not width.
- **Ranking grid:** `repeat(auto-fit, minmax(320px, 1fr))` — cards wrap before they shrink.
- **Report pages:** single-column flow of `{component.card}` sections; each card answers one question.

### Whitespace Philosophy

Whitespace is the anti-fatigue tool. A politician report may hold ten cards; each gets 24px separation and the page breathes 32px/64px top/bottom. Dense data inside, calm air outside. No decoration in the whitespace — the trust signal is restraint.

## Elevation & Depth

| Level         | Treatment                              | Use                                           |
| ------------- | -------------------------------------- | --------------------------------------------- |
| Flat          | No shadow, no border                   | Page floor, hero                              |
| Soft hairline | 1px `{colors.line}` border             | Cards, inputs, tables, inactive pills         |
| Color block   | Full-bleed signature surface, no shadow| Signature green / purple / dark bands         |
| Dashed hairline | 1px dashed `{colors.line}`           | Degraded/unlinked states ("dados não ligados")|

The philosophy is **color-block first, shadow second**. No soft-glow, no heavy elevation. Depth comes from white cards on the neutral-100 floor and from signature bands breaking the scroll.

## Shapes

### Border Radius Scale

| Token          | Value  | Use                                                    |
| -------------- | ------ | ------------------------------------------------------ |
| `{rounded.sm}` | 4–6px  | Trend bars (top only), marker badges, attention badges |
| `{rounded.md}` | 8–12px | Cards, explainer boxes, result-row hover               |
| `{rounded.pill}` | 999px | Search input, toggle pills — the interactive, touchable layer |

### Data Geometry

Trend bars are square-shouldered (radius only on top corners), flex-grown with 6px gaps, `{colors.accent-soft}` fill. Ranking bars are 8px tall, `{colors.accent}`, `{rounded.sm}`. Numbers never wrap (`white-space: nowrap` + tabular-nums on `.num` cells).

## Components

### Buttons & Pills

**`button-primary`** — Background `{colors.accent}`, white text, `{rounded.md}` (12px), `{typography.button}` 16px/500. One per viewport. Active darkens to `{colors.accent-hover}`.

**`pill-toggle`** — Year and house selectors. Inactive: white bg, 1px `{colors.line}`, ink text. Active: `{colors.accent}` bg, white text, no border. Rounded `{rounded.pill}`. Pills are the only pill-shaped elements with `{rounded.pill}` besides search — they signal "switchable view."

**`text-link`** — Inline links inherit ink; warning-context links use `{colors.warn}` with a dotted underline. No link-blue in this system — green is the action color.

### Cards & Containers

**`card`** — The atomic report unit. White bg, 1px `{colors.line}`, `{rounded.md}` (12px), 20px padding, 24px top margin. Header is `{typography.label-caps}` muted uppercase + a `{component.source-line}` right-aligned.

**`hero-search`** — White canvas, centered. h1 `{typography.display-lg}`, one-line explainer in muted, then the pill search input (min(480px, 100%), 12px×20px padding, `{rounded.pill}`, hairline border). No backdrop art — the search bar IS the hero.

**`stat-trio`** — Three flexed stats (empenhado / pago / % pago) at `{typography.display-md}` tabular-nums, 24px gaps, caption under each. The money story in one glance.

**`big-number`** — 34px/700 tabular-nums headline figure with a `{typography.caption}` sub-line. One per card maximum.

**`explainer`** — Plain-language box: `{colors.accent-soft}` at 30% color-mix over white, `{rounded.md}` (8px), 10px×14px padding, `{typography.body}` at 14px. Carries the civic-education load ("O que é empenhado?") without leaving the flow.

**`tooltip-term`** — Inline dotted-underline term (`{colors.muted}` underline), `cursor: help`. Jargon gets defined in place, never in a footnote.

**`source-line`** — "Fonte: Dados Abertos da Câmara" in `{typography.meta}` muted, right-aligned in card headers. Trust furniture on every data surface.

### Data Display

**`trend-bars`** — Monthly spending bars, 64px tall, `{colors.accent-soft}` fill, min-height 2px so zero months stay visible, caption labels below. One glance = rhythm of the year.

**`data-table`** — Largest-items table. `{typography.body-sm}` cells, muted 13px headers, hairline row dividers, right-aligned `.num` columns with tabular-nums. Item subtext at `{typography.meta}`.

**`ranking-grid`** — Home rankings: 320px-min card grid, each row pairing a name with an 8px `{colors.accent}` bar in a 38%-width cell. Bars make rank legible before numbers are read.

**`badge-attention`** — Above-median expense flag. `{colors.warn}` text on `{colors.warn-soft}` 35% color-mix fill, `{rounded.sm}` (6px), 12px. The ONLY red on the page — scarcity makes it meaningful.

**`rollup-attention`** — Card-level attention summary line, 14px, with a dotted-underline `{colors.warn}` link jumping to the flagged item.

**`badge-pix`** — Permanent Emenda PIX type marker: `{colors.marker}` text on `{colors.marker-bg}` with a 1px `{colors.marker-line}` border, `{rounded.sm}` (4px), `{typography.badge}`. Yellow = category, not alarm. This is the flag-yellow moment, kept small and factual.

**`result-row`** — Search results: flex row, name + UF muted right, hairline divider, hover fill `{colors.accent-soft}` at 30% with `{rounded.md}`. Whole row is the link target.

**`not-linked`** — Degraded state card: 1px dashed `{colors.line}`, `{rounded.md}`, muted 14.5px text. Honest emptiness, styled like a placeholder not an error.

### Signature Bands

**`signature-green-band`** — Full-bleed `{colors.signature-green}` with white `{typography.display-md}` headline and one short paragraph. Used once per report page for the "where the money goes" story. CTA inside is a white-bg button (never translucent-on-dark).

**`signature-purple-band`** — Same shape, `{colors.signature-purple}`. Reserved for methodology and cross-chamber comparison moments so purple keeps its "second perspective" meaning.

## Do's and Don'ts

### Do

- Let big numbers lead. Weight 700 belongs to data; prose stays 400.
- Use green (`{colors.accent}`) for every action and primary data series — it is the flag's field and the product's voice.
- Keep yellow small and factual: PIX badges, highlight rules, glossary accents.
- Trust whitespace between cards. Ten calm cards beat one dense wall.
- Put a `{component.source-line}` on every data surface. Trust is the brand.
- Define every jargon term inline with `{component.explainer}` or `{component.tooltip-term}`.
- Anchor report sections with `{spacing.section}` (64px) rhythm.
- Use `color-mix` tints for soft fills so the palette stays the only place with raw hexes.

### Don't

- Don't use red decoratively. Red = "this deserves your attention," full stop. Decorative red trains users to ignore real warnings.
- Don't fill heroes or large backgrounds with yellow — flag yellow at scale fatigues. Marker-size only.
- Don't add gradients, mesh, or aurora backdrops. White canvas, always.
- Don't bold prose or labels to 700 — that weight is reserved for numbers and h1.
- Don't introduce a link-blue. Green is the action color; links inherit ink.
- Don't use `{rounded.pill}` outside search inputs and toggle pills.
- Don't stack two signature bands back-to-back — the rhythm is white → signature → white.
- Don't render numeric columns without tabular-nums. Jittering digits destroy trust in data.
- Don't repeat the same surface mode in consecutive bands.

## Responsive Behavior

### Breakpoints

| Name    | Width      | Key Changes                                                                                  |
| ------- | ---------- | -------------------------------------------------------------------------------------------- |
| Mobile  | < 640px    | Single column; ranking grid 1-up; stat-trio wraps; search input full-width; wrap padding 20px |
| Tablet  | 640–1024px | Ranking grid 2-up; tables keep hairlines, may scroll horizontally                            |
| Desktop | > 1024px   | Wrap caps at 860px; ranking grid auto-fits; whitespace does the framing, not wider columns    |

### Touch Targets

- Pills and buttons ≥ 44px tall (6px×16px padding on 14–16px type clears it).
- Search input ≥ 44px tall, full-thumb-width on mobile.
- Result rows pad to ≥ 40px effective tap height.

### Collapsing Strategy

- Cards and grids reduce columns; they never shrink fonts below `{typography.meta}` (12px).
- Tables scroll horizontally rather than reflowing numeric columns.
- `stat-trio` wraps to stacked stats on mobile, preserving order empenhado → pago → %.

## Iteration Guide

1. Focus on ONE component at a time. Reference its key directly (`{component.badge-pix}`, `{colors.accent-soft}`).
2. Raw hexes appear ONLY in `globals.css` layer 1 (palette). Components reference semantic tokens; soft fills use `color-mix` on tokens.
3. Before adding a component, ask which question it answers for the citizen. If none, it doesn't ship.
4. New semantic meaning → new semantic token in layer 2, never a raw color in layer 3.
5. When in doubt about emphasis: bigger number before bolder text, signature band before accent flood, whitespace before decoration.
6. After token edits, verify contrast: `{colors.warn}` on warn-soft tints, `{colors.marker}` on `{colors.marker-bg}`, white on `{colors.accent}` all must clear 4.5:1 for text.

## Known Gaps

- Dark mode is not yet defined; when added, override semantic tokens only — the palette layer stays fixed.
- Chart palette beyond two series (green/purple) is unformalized; a third series would draw yellow-700 before any new hue.
- Empty-state illustration style is undefined (currently text-only `not-linked` / `empty-note`).
- Animation/transition timings not in scope; current behavior is instant state change.
- Senado-specific theming (purple accent swap) is described conceptually but not implemented.
