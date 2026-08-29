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
---

# GMWallet Asset Atlas

## Overview

A dense, scannable asset explorer for developers and wallet operators. Use calm neutral surfaces,
restrained blue interaction color, equal light/dark hierarchy, Simplified Chinese and English, and
shadcn UI primitives.

## Colors

Use the matching background, surface, text, muted, and border tokens for each theme. Reserve
`{colors.primary}` for focus, selection, links, and progress; use `{colors.danger}` for errors.
Text and controls must meet WCAG AA contrast.

## Typography

Use the system-first Inter stack. Apply `{typography.heading}` to titles, `{typography.body}` to UI
copy, `{typography.label}` to filters and metadata, and `{typography.data}` to identifiers, URLs,
code, and tabular counts.

## Layout

Use a 4px rhythm and a 1536px maximum content width. The icon grid uses responsive columns at least
136px wide. Below 768px, filters wrap, touch targets remain at least 44px, details use a bottom
sheet, and content may truncate but never overflow. Documentation uses a 960px reading column.

## Elevation & Depth

Use surface tint and one-pixel borders before shadows. Cards may lift 2px on hover; dialogs may use
a stronger shadow and blurred overlay. Avoid nested shadows and decorative glass layers.

## Shapes

Controls use `{rounded.md}`, cards and dialogs use `{rounded.lg}`, and identities use
`{rounded.full}`. Borders are one pixel. Use Lucide icons and never stretch the GMWallet mark.

## Components

Use shadcn UI primitives. `CryptoIdentity` exclusively renders crypto assets and owns loading,
fallbacks, background adaptation, and badges. Cards require hover and keyboard-focus states;
loading, empty, and error states must remain explicit.

Header controls are matching 36px buttons with 18px icons. The header enters fixed position after
about 160px, never shows a bottom border or shadow, and keeps a 64px layout placeholder. The filter
toolbar stays beneath it and also has no divider or shadow. When both are sticky, render one shared,
dynamically sized translucent backdrop instead of two overlapping blur layers. Search remains
visible and supports Command/Ctrl+K.

Keep motion within 150–200ms and honor reduced-motion preferences. Dialogs close with Escape and
restore focus.
