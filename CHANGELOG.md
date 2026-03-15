# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [Unreleased]

### Added

- Open source governance docs (`CONTRIBUTING.md`, `VERSIONING.md`, `NOTICE.md`, `LICENSE`)
- documentation requirements for Quran dataset integrity and attribution
- full dataset pipeline scripts:
  - `scripts/import` (Tanzil text + metadata -> `data/quran.json`)
  - `scripts/validate` (`6236` count, uniqueness, field/range checks)
  - `scripts/seed_relations` (starter mutashabihat examples)

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
