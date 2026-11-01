#!/usr/bin/env bash
set -euo pipefail
dotnet run --project tests/PosterImageUrl.Tests/PosterImageUrl.Tests.csproj --configuration Release
dotnet build Jellyfin.Plugin.Stash/Stash.csproj --configuration Release.Emby --nologo
output=Jellyfin.Plugin.Stash/bin/Release.Emby/netstandard2.1
package=artifacts/emby-stash-auto-poster
mkdir -p "$package"
cp "$output/Stash.dll" "$package/Emby.Plugins.Stash.dll"
cp "$output/Stash.pdb" "$package/Emby.Plugins.Stash.pdb"
cp LICENSE README.md docs/proxy-contract.md "$package/"
mkdir -p "$package/docs" "$package/proxy"
cp docs/proxy-contract.md "$package/docs/"
cp proxy/README.md "$package/proxy/"
tar -czf artifacts/Emby.Plugins.Stash-auto-poster.tar.gz -C "$package" .
sha256sum "$package/Emby.Plugins.Stash.dll" > artifacts/SHA256SUMS
printf 'Build complete: %s\n' "$package/Emby.Plugins.Stash.dll"
