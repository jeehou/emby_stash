#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$project_root/.build/nuget" "$project_root/artifacts"
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --mount "type=bind,source=$project_root,target=/workspace" \
  --workdir /workspace \
  --env DOTNET_CLI_HOME=/tmp/dotnet-cli \
  --env NUGET_PACKAGES=/workspace/.build/nuget \
  --env DOTNET_CLI_TELEMETRY_OPTOUT=1 \
  --env DOTNET_NOLOGO=1 \
  "${DOTNET_SDK_IMAGE:-mcr.microsoft.com/dotnet/sdk:10.0}" \
  bash scripts/container-build.sh
