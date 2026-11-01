#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
docker build --tag emby-stash-poster-proxy:local "$project_root/proxy"
mkdir -p "$project_root/artifacts/proxy"
build_container="$(docker create emby-stash-poster-proxy:local)"
trap 'docker rm "$build_container" >/dev/null' EXIT
docker cp "$build_container:/usr/local/bin/stash-poster-proxy" "$project_root/artifacts/proxy/stash-poster-proxy"
mkdir -p "$project_root/artifacts/proxy/licenses"
docker cp "$build_container:/usr/share/licenses/stash-poster-proxy/." "$project_root/artifacts/proxy/licenses/"
cp "$project_root/proxy/README.md" "$project_root/artifacts/proxy/README.md"
sha256sum "$project_root/artifacts/proxy/stash-poster-proxy"
