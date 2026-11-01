# Plugin thumbnail

Original artwork for this project, covered by the repository MIT license.
The editable source is `plugin-thumb.svg`; the committed 640 × 360 PNG is embedded
in the Emby DLL as `Stash.Assets.plugin-thumb.png` and served by `IHasThumbImage`.
No external image host is required. This is a project-specific icon, not an
official Stash or Emby logo.

To regenerate the PNG, install librsvg (`rsvg-convert`) and the Lato font, then run
from the repository root:

```sh
rsvg-convert Jellyfin.Plugin.Stash/Assets/plugin-thumb.svg \
  -o Jellyfin.Plugin.Stash/Assets/plugin-thumb.png
```

Emby interface: https://dev.emby.media/reference/pluginapi/MediaBrowser.Common.Plugins.IHasThumbImage.html
