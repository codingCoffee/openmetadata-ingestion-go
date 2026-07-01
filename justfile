# Release automation. GoReleaser runs inside docker so nothing needs to be
# installed locally except docker + git.
#
#   just release patch    # v1.2.3 -> v1.2.4
#   just release minor    # v1.2.3 -> v1.3.0
#   just release major    # v1.2.3 -> v2.0.0
#
# Requires a GITHUB_TOKEN (with `repo` scope) in the environment for publishing.

# Pinned goreleaser image. Bump deliberately.
goreleaser_image := "goreleaser/goreleaser:v2.12.7"

_default:
    @just --list

# Bump the semver tag ({{bump}} = patch|minor|major), push it, then release.
release bump:
    #!/usr/bin/env bash
    set -euo pipefail

    case "{{bump}}" in
        patch|minor|major) ;;
        *) echo "usage: just release patch|minor|major" >&2; exit 1 ;;
    esac

    # A dirty tree would be baked into the release build; refuse it.
    if [ -n "$(git status --porcelain)" ]; then
        echo "working tree is dirty; commit or stash first" >&2
        exit 1
    fi

    current="$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"
    read -r major minor patch <<<"$(echo "${current#v}" | tr '.' ' ')"

    case "{{bump}}" in
        patch) patch=$((patch + 1)) ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        major) major=$((major + 1)); minor=0; patch=0 ;;
    esac
    next="v${major}.${minor}.${patch}"

    echo "releasing ${current} -> ${next}"
    git tag -a "${next}" -m "release ${next}"
    git push origin "${next}"

    just _goreleaser release --clean

# Build a local snapshot release without tagging or publishing (for testing).
snapshot:
    just _goreleaser release --clean --snapshot

# Validate the .goreleaser.yaml.
check:
    just _goreleaser check

# Run goreleaser inside docker with the repo mounted.
# The GIT_CONFIG_* vars mark the mounted repo as safe (it is owned by the host
# user, not root, so git would otherwise refuse it as "dubious ownership").
_goreleaser *args:
    docker run --rm \
        -v "$(pwd):/workspace" \
        -w /workspace \
        -e GITHUB_TOKEN \
        -e GIT_CONFIG_COUNT=1 \
        -e GIT_CONFIG_KEY_0=safe.directory \
        -e GIT_CONFIG_VALUE_0='*' \
        {{goreleaser_image}} {{args}}
