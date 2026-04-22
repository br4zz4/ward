# Publish ward via Homebrew, APT, and APK

> TLDR: Create a public GitHub repo, set up GoReleaser to produce versioned binaries, and wire up distribution via Homebrew tap, APT (Packagecloud), and APK (Alpine aports).

**Status:** completed
**Created:** 2026-04-21
**Owner:** @oporpino

---

## Context

`ward` is currently only installable via `go install` or building from source. To reach a wider audience (macOS users via Homebrew, Debian/Ubuntu via APT, Alpine/Docker users via APK), we need a proper release pipeline and distribution setup.

## Objectives

- Publish the source to a public GitHub repo (`oporpino/ward`)
- Automate cross-platform binary releases with GoReleaser + GitHub Actions
- Distribute via Homebrew tap (`oporpino/homebrew-tap`)
- Distribute via APT through Packagecloud (free tier)
- Distribute via APK through an Alpine aports PR

## Changes

### 1. Create public GitHub repository
- `gh repo create oporpino/ward --public --source=. --push`
- Add license (MIT) if not present

### 2. GoReleaser setup
- Add `.goreleaser.yaml` at repo root
  - Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
  - Produces: tarballs + checksums + Homebrew formula update + APT `.deb` packages
- Add `.github/workflows/release.yml`
  - Triggered on `git tag v*`
  - Runs `goreleaser release`
  - Publishes GitHub Release with all artifacts

### 3. Homebrew tap
- `gh repo create oporpino/homebrew-tap --public`
- GoReleaser auto-commits the updated formula to `Formula/ward.rb` on each release
- Users install with: `brew tap oporpino/tap && brew install ward`

### 4. APT (Packagecloud)
- Create free account at packagecloud.io under `oporpino`
- Add GoReleaser `publisher` step to push `.deb` packages to Packagecloud
- Store `PACKAGECLOUD_TOKEN` as a GitHub Actions secret
- Users install with:
  ```sh
  curl -s https://packagecloud.io/install/repositories/oporpino/ward/script.deb.sh | sudo bash
  sudo apt install ward
  ```

### 5. APK (Alpine Linux aports)
- Write `APKBUILD` file (Alpine package spec)
- Submit PR to [alpinelinux/aports](https://gitlab.alpinelinux.org/alpine/aports) under `community/ward`
- This is a manual, one-time contribution — Alpine maintainers review and merge
- Users install with: `apk add ward` (once accepted)

### 6. Update README
- Replace current `go install` instruction with install sections for each method

## How to verify

- `brew tap oporpino/tap && brew install ward && ward --version`
- On Debian/Ubuntu: follow Packagecloud script, `apt install ward && ward --version`
- On Alpine: `apk add ward && ward --version` (after aports merge) or test APKBUILD locally with `abuild`

## Documentation

No documentation changes needed.
