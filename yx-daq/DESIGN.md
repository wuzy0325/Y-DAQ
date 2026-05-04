---
name: YX-DAQ Neon
description: >-
  Dark neon cyberpunk design system for YX-DAQ data acquisition application.
  Glass-morphism cards, neon glow effects, and holographic surfaces on deep-space backgrounds.
colors:
  primary: "#b829ff"
  primary-dark: "#a820f0"
  primary-light: "#d966ff"
  accent: "#00f5ff"
  accent-dark: "#00d4e0"
  accent-light: "#66faff"
  success: "#00ff88"
  success-dark: "#00e07a"
  warning: "#ffaa00"
  warning-dark: "#e69900"
  danger: "#ff3366"
  danger-dark: "#e62e5c"
  info: "#00aaff"
  info-dark: "#0099e6"
  bg-primary: "#0a0a1a"
  bg-secondary: "rgba(255,255,255,0.06)"
  bg-tertiary: "rgba(255,255,255,0.03)"
  text-primary: "#ffffff"
  text-secondary: "rgba(255,255,255,0.80)"
  text-tertiary: "rgba(255,255,255,0.60)"
  text-muted: "rgba(255,255,255,0.40)"
  chart-line-1: "{colors.primary}"
  chart-line-2: "{colors.accent}"
  chart-line-3: "{colors.success}"
  chart-line-4: "{colors.warning}"
typography:
  body-md:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0px
  body-lg:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0px
  heading-sm:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 14px
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: 0.5px
  heading-md:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 16px
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: 0.5px
  heading-lg:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 20px
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: 0px
  mono-md:
    fontFamily: "'Microsoft YaHei', 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace"
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: 0px
  label-xs:
    fontFamily: "'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
    fontSize: 11px
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: 0.3px
spacing:
  xs: 4px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
  2xl: 32px
  3xl: 48px
rounded:
  sm: 8px
  md: 12px
  lg: 16px
  xl: 20px
  2xl: 24px
components:
  glass-card:
    backgroundColor: "{colors.bg-secondary}"
    textColor: "{colors.text-secondary}"
    rounded: "{rounded.md}"
    padding: "{spacing.lg}"
  button-primary:
    backgroundColor: "rgba(184,41,255,0.15)"
    textColor: "{colors.primary}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm} {spacing.lg}"
  button-success:
    backgroundColor: "rgba(0,255,136,0.15)"
    textColor: "{colors.success}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm} {spacing.lg}"
  button-warning:
    backgroundColor: "rgba(255,170,0,0.15)"
    textColor: "{colors.warning}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm} {spacing.lg}"
  button-danger:
    backgroundColor: "rgba(255,51,102,0.15)"
    textColor: "{colors.danger}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm} {spacing.lg}"
  input-field:
    backgroundColor: "rgba(0,0,0,0.3)"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm} {spacing.md}"
  value-display:
    typography: "{typography.heading-md}"
    textColor: "{colors.accent}"
  device-light-online:
    backgroundColor: "{colors.success}"
    rounded: full
  device-light-acquiring:
    backgroundColor: "{colors.accent}"
    rounded: full
  chart-line:
    textColor: "{colors.text-tertiary}"
  sidebar:
    backgroundColor: "{colors.bg-secondary}"
    textColor: "{colors.text-secondary}"
    rounded: "{rounded.md}"
    padding: "{spacing.lg}"
---

# YX-DAQ Neon

## Overview

YX-DAQ Neon is a **dark cyberpunk / neon-tech** design system purpose-built for industrial data acquisition software. The aesthetic combines deep-space backgrounds (`#0a0a1a`) with vibrant neon accents and glass-morphism surfaces. It communicates precision, technological sophistication, and real-time data awareness.

### Brand personality

- **Precision** — clean typography, generous whitespace, sharp data presentation
- **Technology** — neon glow effects, holographic glass panels, subtle grid backgrounds
- **Immersion** — dark backgrounds reduce eye strain during extended monitoring sessions

## Colors

### Brand colors

- **Primary (Neon Purple)** `#b829ff` — the hero colour. Used for primary interactive elements, focus indicators, and chart lines.
- **Accent (Neon Cyan)** `#00f5ff` — secondary interactive colour and live data indicators. Pairs with primary for gradient transitions.
- **Success (Neon Green)** `#00ff88` — connected status, acquisition active, healthy values.
- **Warning (Neon Orange)** `#ffaa00` — caution states, approaching limits.
- **Danger (Neon Red)** `#ff3366` — critical alerts, disconnection, error states.
- **Info (Neon Blue)** `#00aaff` — informational indicators.

### Background hierarchy

| Layer | Token | Usage |
|-------|-------|-------|
| Deep space | `bg-primary` | Page backgrounds, main canvas |
| Glass panel | `bg-secondary` | Cards, sidebars, elevated surfaces |
| Subtle | `bg-tertiary` | Hover states, subtle contrast |

### Text hierarchy

| Level | Token | Opacity | Usage |
|-------|-------|---------|-------|
| Primary | `text-primary` | 100% white | Headings, main content |
| Secondary | `text-secondary` | 80% white | Body text, card content |
| Tertiary | `text-tertiary` | 60% white | Labels, metadata |
| Muted | `text-muted` | 40% white | Placeholders, disabled |

### Chart colors

Chart lines follow a fixed 4-colour rotation: primary purple (line 1), accent cyan (line 2), success green (line 3), warning orange (line 4). Each line also carries a subtle neon glow (`shadowBlur: 4`) in its own colour.

## Typography

### Font stack

- **UI text:** Microsoft YaHei (for Chinese glyphs), falling back to system sans-serif.
- **Monospace:** Microsoft YaHei for Chinese, SF Mono / Cascadia Code / Consolas for numerical data display.

### Type scale

| Token | Size | Weight | Usage |
|-------|------|--------|-------|
| `label-xs` | 11px | 500 | Channel names, tags, badges |
| `body-md` | 14px | 400 | Default body text, table cells |
| `mono-md` | 13px | 400 | Numerical values, data readouts |
| `heading-sm` | 14px | 600 | Card titles, section headers |
| `body-lg` | 16px | 400 | Enlarged content, descriptions |
| `heading-md` | 16px | 600 | Panel headings, dialog titles |
| `heading-lg` | 20px | 600 | Page titles, prominent headings |

### Guidelines

- Use `mono-md` for all numerical data values, channel readings, and measurement displays.
- Use `label-xs` for secondary channel labels and status tags.
- Use `heading-sm` as the default card header. Content text uses `body-md`.

## Layout & Spacing

### Spacing scale

A 4px base unit with a 12-16-24 rhythm for common UI gaps:

- **xs (4px):** Tight inner gaps, icon spacing
- **sm (8px):** Element-to-element gaps, input padding
- **md (12px):** Card inner padding, form field spacing
- **lg (16px):** Card-to-card gaps, section padding
- **xl (24px):** Major section dividers, dialog padding
- **2xl (32px):** Page-level margins
- **3xl (48px):** Large screen gutters

### Layout conventions

- Sidebar: fixed 220px width, min-width 220px
- Device list items: `padding: 10px 12px` with 4px gap
- Chart area: flex-1 with `50px left / 20px right / 30px top / 30px bottom` grid margins
- Glass card row gap: `lg` (16px)
- Channel value grid: items at 130px width, text-align center, wrap with 8px gap

### Dashboard layout

```
├── sidebar (220px) ───┤─── data-area (flex-1) ───┤
│  sidebar-title        │  chart-card (flex-1)     │
│  acq-controls         │  channel-card (shrink)   │
│  device-list (scroll) │                          │
├───────────────────────┤──────────────────────────┤
```

- Dashboard height: `calc(100vh - topbar - statusbar)`
- `acq-controls` row: flex row, 8px gap, `padding: 8px 12px`, bottom border
- `chart-card` inside data-area: `flex: 1; min-height: 0` to allow shrink
- `channel-card` below chart: `flex-shrink: 0`

### Device view layout

- Device list / device detail: two-panel split with `flex` + `gap: lg`
- Channel config table: full width, no horizontal scroll
- Motion axis grid: 2 columns, card-based, each axis as GlassCard

### Form layout

- Label aligned left, `margin-bottom: sm` above input
- Inline controls in a row: `display: flex; gap: sm; align-items: center`
- Dialog body padding: `lg` (16px) horizontal, `md` (12px) vertical

## Elevation & Depth

### Shadow system

| Level | Usage |
|-------|-------|
| `shadow-sm` `0 2px 8px rgba(0,0,0,0.3)` | Low-elevation elements |
| `shadow-md` `0 4px 16px rgba(0,0,0,0.4)` | Default card elevation |
| `shadow-lg` `0 8px 32px rgba(0,0,0,0.5)` | Modals, dialogs |

### Glass-morphism shadows

Glass cards use a layered shadow with an inner highlight to simulate frosted glass:
- **Default:** `0 8px 32px rgba(0,0,0,0.4), inset 0 1px 0 rgba(255,255,255,0.1)`
- **Hover:** `0 12px 48px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.15)`

### Neon glow effects

Applied to interactive and status elements:
- **Primary glow:** `0 0 20px rgba(184,41,255,0.5), 0 0 40px rgba(184,41,255,0.3)`
- **Accent glow:** `0 0 20px rgba(0,245,255,0.5), 0 0 40px rgba(0,245,255,0.3)`
- **Success glow:** `0 0 20px rgba(0,255,136,0.5), 0 0 40px rgba(0,255,136,0.3)`

### Blur effects

| Level | Value | Usage |
|-------|-------|-------|
| `blur-sm` | `8px` | Subtle glass blur |
| `blur-md` | `16px` | Standard glass blur |
| `blur-lg` | `24px` | Heavy overlay blur |

## Shapes

### Corner radii

- **sm (8px):** Buttons, inputs, device items, status tags
- **md (12px):** Glass cards, sidebars, modal dialogs
- **lg (16px):** Large panels, elevated containers
- **xl (20px):** Special decorative elements
- **2xl (24px):** Maximum rounding for accent elements

## Components

### GlassCard

The fundamental surface component. Features frosted-glass translucency with subtle border glow on hover.

```
.glass-card {
  background: rgba(255,255,255,0.06);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 12px;
  padding: 16px;
  backdrop-filter: blur(16px);
}
```

- Header has a bottom border (`rgba(255,255,255,0.06)`) with card title in `heading-sm`.
- Elevated variant uses `rgba(255,255,255,0.08)` background with hover border glow.
- On hover, border colour shifts to `rgba(184,41,255,0.2)`.

### Buttons

Element Plus buttons with transparent glass backgrounds and neon text:

- Each variant uses its colour with 15% opacity background and matching text.
- Default button: transparent bg with white text.
- Hover: fills with the variant colour and inverts text to black.
- All buttons use `2px` border-radius (Element Plus default), uppercase text transformation, and bold weight.

### ValueDisplay

Used for numerical data readouts. Value text uses `heading-md` size in `accent` colour with a mono-style numeral presentation. A `--` placeholder appears when no data is available.

### Device indicator lights

Three status lights as small (10px) circles:
- **Connected:** `success` fill with success glow
- **Acquiring:** `accent` fill with accent glow + pulse animation
- **Disconnected:** 30% white, no glow

### Tables (Element Plus overrides)

- Header row: 5% white background with bottom border
- Body rows: transparent with 5% accent border separator
- Hover row: `rgba(0,242,255,0.08)` highlight
- Current row: `rgba(0,242,255,0.12)` highlight

## Do's and Don'ts

### Do

- Do use `colors.primary` (neon purple) for focus indicators, active selections, and primary interactive elements.
- Do use `colors.accent` (neon cyan) for live/streaming indicators and secondary actions.
- Do use `colors.success` for "connected" and "acquiring" status indicators only.
- Do use `colors.danger` exclusively for disconnection, errors, and emergency stop states.
- Do apply glass-morphism (`bg-secondary` + blur) to all elevated surfaces.
- Do use `spacing.md` (12px) as the minimum gutter between related elements.
- Do use `typography.mono-md` for all numeric measurement displays.
- Do use `spacing.lg` (16px) as the standard gap between independent cards and sections.
- Do apply the 4-colour chart rotation in order: primary → accent → success → warning.
- Do keep `::-webkit-scrollbar` width at 4px with transparent track.

### Don't

- Don't mix neon colours as solid backgrounds — always use them as text, borders, or glows on dark surfaces.
- Don't use `colors.primary` for disabled elements — use `text-muted` instead.
- Don't apply glow effects to more than one interactive element at the same level.
- Don't use solid white backgrounds for any surface — always use dark glass variants.
- Don't change the scrollbar width from 4px.
- Don't use rounded corners larger than `rounded.md` (12px) for interactive elements.
- Don't use `!important` in component styles — use proper CSS specificity or Element Plus variable overrides.
- Don't use emoji for UI icons — use SVG or Element Plus icon components instead (emoji in code comments only).
- Don't hardcode chart colours — always use the `chart-line-{1..4}` token rotation.
- Don't let flex items overflow — always add `min-width: 0` / `min-height: 0` on flex children with `overflow: hidden`.
- Don't use fixed heights for scrollable areas — use `flex: 1; min-height: 0; overflow-y: auto`.
- Don't mix `px` spacing with token spacing — use `var(--spacing-*)` or SCSS `$spacing-*` consistently.
- Don't use `width: 100%` on flex children inside `flex: 1` parents — use `flex: 1` or `align-self: stretch`.
