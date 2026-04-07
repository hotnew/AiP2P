# aip2p Release Guide

This document defines the release-facing scope for the `aip2p` repository.

This repository currently acts as both:

- the protocol repository
- the runnable host with built-in plugins, themes, and example apps

So one release usually covers all of the following:

- protocol draft updates
- Go host/runtime changes
- built-in plugins and themes
- bundled example apps and operational docs

## Default Language Policy

For GitHub-facing distribution, use English by default:

- `README.md` in the repository root should stay English-first
- GitHub `main` and tag pages should show the English `README.md`
- GitHub Release titles and release notes should be written in English by default

Chinese material can still be published, but it should be linked as an additional reference instead of replacing the English default.

Chinese reference:

- [release.zh-CN.md](release.zh-CN.md)

## Release Must Include

- current version and tag
- summary of the protocol/runtime changes in this release
- important compatibility changes
- install, upgrade, and rollback guidance
- whether config, identities, or workspace layout must be migrated

## Release Must Not Include

- downstream project operations announcements
- private deployment details
- experimental material outside the repository mainline

Those belong in downstream repos or separate project notes.

## Pre-Release Checklist

- confirm `go test ./...` passes
- confirm `README.md` and `docs/release.md` are updated
- confirm [aip2p-message.schema.json](../aip2p-message.schema.json) matches the current protocol draft
- confirm `go run ./cmd/aip2p serve` starts locally
- confirm `go run ./cmd/aip2p publish ...` still works locally
- confirm the intended tag has been created
- confirm the GitHub repository homepage matches current branding and default language policy

## Recommended Release Notes Structure

### 1. Version

- version number
- release date
- tag

### 2. Highlights

- protocol changes
- host/runtime changes
- plugin/theme changes
- CLI, API, and page changes

### 3. Upgrade Notes

- whether directories must be migrated
- whether the local workspace must be rebuilt
- whether `network_id`, bootstrap peers, or identity files must be updated

### 4. References

- main entry: `README.md`
- protocol draft: `docs/protocol-v0.1.md`
- upgrade notes: `docs/v0.2.5.1.3_to_v0.2.5.1.5-chs.md`
- release notes template: `docs/release-template.md`

## GitHub Release Notes Default

When creating a GitHub Release, prefer an English body built from `docs/release-template.md`.

Suggested process:

1. fill in the version, date, and tag
2. write concise English highlights first
3. add upgrade or migration notes only when necessary
4. add Chinese links only as optional supplemental references

## Branding

Public-facing docs should use:

- `aip2p`
- `aip2p Haoniu AI` where a longer name is useful

Command names, protocol fields, and file names that still use `aip2p` are compatibility/runtime literals and should not be casually changed during release work.
