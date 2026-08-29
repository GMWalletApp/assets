---
version: "1.0"
name: GMWallet Asset Atlas
description: A precise, high-density asset explorer with metallic neutrals and restrained electric-blue interaction color.
colors:
  primary: "#2563EB"
  primary-hover: "#1D4ED8"
  background: "#F7F8FA"
  surface: "#FFFFFF"
  surface-muted: "#F0F2F5"
  text: "#111318"
  muted: "#667085"
  border: "#DDE1E7"
  on-primary: "#FFFFFF"
  danger: "#DC2626"
  dark-background: "#080A0F"
  dark-surface: "#10131A"
  dark-surface-muted: "#181C24"
  dark-text: "#F4F6F8"
  dark-muted: "#98A2B3"
  dark-border: "#252A34"
typography:
  heading:
    fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif"
    fontSize: 24px
    fontWeight: 650
    lineHeight: 1.25
    letterSpacing: -0.02em
  body:
    fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif"
    fontSize: 12px
    fontWeight: 600
    lineHeight: 1.35
  data:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: 12px
    fontWeight: 500
    lineHeight: 1.5
rounded:
  sm: 6px
  md: 10px
  lg: 16px
  full: 999px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
components:
  page:
    backgroundColor: "{colors.background}"
    textColor: "{colors.text}"
    typography: "{typography.body}"
  dark-page:
    backgroundColor: "{colors.dark-background}"
    textColor: "{colors.dark-text}"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    rounded: "{rounded.lg}"
    padding: "{spacing.md}"
  dark-card:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.lg}"
  muted-card:
    backgroundColor: "{colors.surface-muted}"
    rounded: "{rounded.md}"
  dark-muted-card:
    backgroundColor: "{colors.dark-surface-muted}"
    rounded: "{rounded.md}"
  separator:
    backgroundColor: "{colors.border}"
    height: "1px"
  dark-separator:
    backgroundColor: "{colors.dark-border}"
    height: "1px"
  primary-button:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "{spacing.sm}"
  primary-button-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "{colors.on-primary}"
  supporting-text:
    textColor: "{colors.muted}"
    typography: "{typography.body}"
  dark-supporting-text:
    textColor: "{colors.dark-muted}"
    typography: "{typography.body}"
  error-message:
    textColor: "{colors.danger}"
    typography: "{typography.body}"
---

# GMWallet Asset Atlas

## Overview

The preview is a precision asset atlas for developers and wallet operators. It combines a calm,
technical surface with a dense but scannable icon grid. Metallic black and silver come from the
GMWallet mark; electric blue is reserved for focus, selection, links, and actionable status. Light
and dark themes are first-class and preserve the same hierarchy rather than mechanically inverting
colors. English and Simplified Chinese are first-class locales; a saved choice overrides browser
language detection. The component contract follows shadcn UI. Surface hierarchy is adapted from the catalog's
Vercel reference and crypto-asset density from its Coinbase reference; these are reverse-engineered
references, not official brand specifications.

## Colors

Light pages use `{colors.background}`, with cards on `{colors.surface}` and secondary controls on
`{colors.surface-muted}`. Dark pages use `{colors.dark-background}`, `{colors.dark-surface}`, and
`{colors.dark-surface-muted}`. Body text and borders switch to their matching dark tokens. Use
`{colors.primary}` only for selection, focus, links, and progress; hover uses
`{colors.primary-hover}`. Error messaging uses `{colors.danger}`. All body text and controls must
meet WCAG AA contrast.

## Typography

Use the system-first Inter stack so Chinese and Latin text render consistently without a blocking
font request. `{typography.heading}` is for page titles and dialog titles. `{typography.body}` is
the default UI text, `{typography.label}` is for filters and metadata, and `{typography.data}` is
for addresses, identifiers, URLs, and code. Prefer tabular numerals for counts.

## Layout

Use a 4px base rhythm and a maximum content width of 1536px. The desktop toolbar stays compact and
the icon grid uses responsive auto-fill columns with a minimum width of 136px. Below 768px, filters
wrap at their natural height, cards retain at least a 44px touch target, and secondary metadata may
truncate but never overflow. Asset details use a bottom sheet on small screens. The documentation
route uses a narrower 960px reading column and all code surfaces must stay within the viewport.

## Elevation & Depth

Hierarchy comes primarily from one-pixel borders, subtle surface tint, and restrained shadows.
Cards may lift by 2px on hover with a soft blue-tinted shadow. Dialogs use a stronger shadow and a
blurred overlay. Avoid glass layers over dense data and avoid multiple nested shadows.

## Shapes

Controls use `{rounded.md}`, asset cards and dialogs use `{rounded.lg}`, and identities remain
circular with `{rounded.full}`. Borders are always one pixel. Icons use Lucide's consistent stroke
geometry. The GMWallet mark must remain square or circular and must not be stretched.

## Components

Use shadcn UI primitives for buttons, inputs, selects, tabs, badges, alerts, dialogs, avatars, and
skeletons. `CryptoIdentity` is the only crypto icon renderer and owns loading, mirror fallback,
dominant-color background, dark-mode adaptation, and corner badges. Asset cards expose a clear
hover and keyboard-focus state. Loading uses localized skeletons or a compact spinner; empty and
error states must explain the next action. Header theme, language, and GitHub controls use matching
36px buttons with 18px icons and the same visual hierarchy. Theme and language icon changes use a
snappy Morphicons transition that honors the user's reduced-motion preference. Dialogs close with
Escape and restore focus.

Motion is limited to 150–200ms color, shadow, and transform transitions. Respect
`prefers-reduced-motion` by removing lifts, pulses, and nonessential transitions.

## Design Rules

- Keep filters and counts visible without dominating the asset grid.
- Use semantic theme tokens instead of hard-coded zinc or slate values.
- Preserve equal information density and contrast in light and dark themes.
- Keep English and Simplified Chinese keys structurally aligned; do not mix localized UI with hard-coded copy.
- Use `CryptoIdentity` everywhere a crypto-related image appears.
- Show long addresses and URLs in a monospaced, copyable detail row.
- Refer to alternate CDN endpoints simply as mirrors, without geographic labels.
