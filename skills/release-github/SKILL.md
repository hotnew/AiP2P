---
name: release-github
description: Use this skill when publishing a small GitHub release for AiP2P and the paired latest.org app repo. It covers version bumps, tests, fresh-clone verification, safe push flow, tag creation, and GitHub release creation without breaking the split-repo layout.
---

# GitHub Release

Use this skill when you need to publish a new small version for:

- `AiP2P`
- `latest.org`

This workspace uses one local root Git repository, but GitHub uses two separate repositories:

- `AiP2P/AiP2P`
- `AiP2P/Latest`

Do not push subdirectories from the root repo directly.

## Required Outcome

For every release:

1. bump the version strings in source and docs
2. update the draft release note file
3. run tests locally
4. verify with a fresh GitHub-style install when needed
5. fresh-clone each GitHub repo into a temp directory
6. copy only the relevant subtree into that temp clone
7. commit with GitHub `noreply` email
8. push `main`
9. create and push the new tag
10. create the GitHub Release from the release note file

## Version Rules

Use small increments only.

Typical pair:

- `AiP2P`: `v0.1.X-draft`
- `latest.org`: `v0.1.Y-demo`

Update these places at minimum:

- `aip2p/README.md`
- `aip2p/docs/install.md`
- `aip2p/docs/release.md`
- `aip2p/docs/github-release-v*.md`
- `latest/cmd/latest/main.go`
- `latest/README.md`
- `latest/docs/install.md`
- `latest/docs/release.md`
- `latest/skills/bootstrap-latest/SKILL.md`
- `latest/docs/github-release-v*.md`

## Test Before Push

Run:

```bash
go test ./...
```

In:

- `aip2p`
- `latest/aip2p`
- `latest`

If the change affects runtime behavior, also do a fresh install style check from GitHub or from a fresh clone.

## Safe Push Flow

Do not publish from the root workspace checkout.

Instead:

1. create temp directories
2. clone `https://github.com/AiP2P/AiP2P.git`
3. clone `https://github.com/AiP2P/Latest.git`
4. copy:
   - local `aip2p/` into the fresh `AiP2P` clone
   - local `latest/` into the fresh `Latest` clone
5. exclude `.git`
6. do not commit built binaries unless the repo already intentionally tracks them

Recommended copy pattern:

```bash
rsync -a --delete --exclude '.git' /local/path/aip2p/ /tmp/push-aip2p/
rsync -a --delete --exclude '.git' /local/path/latest/ /tmp/push-latest/
```

## Commit Identity

GitHub may reject pushes that expose a private email address.

Before commit or amend:

```bash
git config user.name AiP2P
git config user.email <github-id>+AiP2P@users.noreply.github.com
```

If a push is rejected with `GH007`, amend the commit with:

```bash
git commit --amend --no-edit --reset-author
```

Then push again.

## Tag And Release

For each repo:

1. push `main`
2. create or update the tag
3. push the tag
4. create the GitHub release with `gh release create`

Pattern:

```bash
git push origin main
git tag -f <tag>
git push -f origin <tag>
gh release create <tag> --repo <owner/repo> --title "<title>" --notes-file <release-notes-file>
```

## Release Notes

Keep one release note file per version:

- `aip2p/docs/github-release-vX.Y.Z-draft.md`
- `latest/docs/github-release-vX.Y.Z-demo.md`

Each note should include:

- release title
- one-sentence summary
- highlights
- install or upgrade reminder

## After Push

Verify:

- the tag exists on GitHub
- the release page exists
- `main` contains the expected commit
- a fresh install can check out the new tag

## Do Not Forget

- keep the split-repo flow
- keep GitHub `noreply` email
- do not rely on the root workspace Git history for publishing
- update both repos when the feature spans both protocol and app layers
