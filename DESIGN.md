# Design Specification

This document defines the visual and UX direction for HifzLink.

## Goals

- keep Quran memorization flow clear and fast
- keep Arabic ayah as the primary reading element
- show translation as optional secondary aid
- maintain lightweight implementation (HTML, CSS, minimal JS)

## Core Rule

Arabic ayah is always primary.

Translation appears directly under the same ayah block when enabled.

No UI mode should show translation without Arabic.

## Visual Direction

Inspired by Quran.com in clarity and calmness, without cloning.

- neutral warm backgrounds
- clean white surfaces
- restrained teal/green accents
- high contrast text for readability

## Design Tokens

- `--bg: #F7F5F0`
- `--surface: #FFFFFF`
- `--surface-soft: #FAF9F6`
- `--text: #1F2937`
- `--text-muted: #6B7280`
- `--border: #E5E7EB`
- `--accent: #0F766E`
- `--accent-strong: #0B5F59`
- `--focus: #14B8A6`

## Typography

Arabic:

- font: `Amiri Quran` (preferred) or `Noto Naskh Arabic`
- size: `2.1rem` desktop, `1.7rem` mobile
- line-height: `2.0` to `2.2`
- direction: RTL, right-aligned

UI + translation text:

- font: `IBM Plex Sans` (or fallback sans stack)
- body size: `16px`
- translation size: `1rem` to `1.05rem`
- translation line-height: `1.7`

## Ayah Block Structure

Each ayah card should render in this order:

1. ayah reference line (`60:8`, surah name, juz)
2. Arabic ayah text
3. translation text (if language enabled)
4. actions (compare, open related)

Translation text style should be visually secondary:

- lower emphasis than Arabic
- muted color
- slightly smaller than Arabic
- enough spacing from Arabic to avoid mixing lines

## Language Modes

Provide three modes in header toggle:

- `AR` (Arabic only)
- `AR + EN`
- `AR + ID`

Mode applies consistently across pages.

## Page Rules

## Home

- top search input for `surah:ayah`
- language toggle visible near search
- quick links remain simple

## Ayah Page

- Arabic verse large and clear
- translation immediately below Arabic when enabled
- related ayah list uses the same Arabic-then-translation pattern

## Compare Page

- two columns on desktop, one column on mobile
- each side keeps same order:
  - Arabic
  - translation below
- maintain equal visual weight between both ayahs

## Surah Relations Page

- list relation pairs
- optional compact preview: Arabic only by default
- on open/expand, translation appears below Arabic for each ayah

## Juz Relations Page

- same pattern as Surah Relations page

## Accessibility

- preserve full ayah text (no truncation)
- keep WCAG AA contrast for UI text and controls
- visible focus styles for keyboard navigation
- minimum tap target size `44px`

## Responsive Rules

- max content width: `1100px`
- compare view stacks on small screens
- keep Arabic line breaks natural and legible
- avoid cramped mixed Arabic/translation blocks

## Implementation Phases

1. token system + typography
2. reusable components (cards, buttons, inputs, ayah block)
3. page-by-page redesign
4. accessibility and mobile QA
