# Emby Stash：自动海报裁剪代理支持

基于 [DirtyRacer1337/Jellyfin.Plugin.Stash](https://github.com/DirtyRacer1337/Jellyfin.Plugin.Stash)
的本地工程，上游起点 `b1ab78b`，本地插件版本 `1.2.0.4`。
保留插件 ID、配置结构和元数据逻辑，新增可关闭的主海报用途标记，配套 Go 代理完成人脸检测与自动裁剪。

## 已实现

- `EnableAutoPosterCrop`：默认关闭；Emby 自动生成的配置界面和 Jellyfin HTML 配置页均有开关。
- 开启后，Scene 主海报及识别结果海报预览增加 `emby_poster=1`。
- 背景图、演员图、厂商 Logo 保持原地址；不固定左裁或右裁。
- URL 时间戳、编码后的认证参数及片段得到保留，重复标记会合并。
- Docker 构建脚本、URL 协议回归测试、代理对接文档。
- Go 认证反向代理：复用固定版本 MetaTube/Pigo 多角度人脸检测，主海报自动裁成 2:3，无脸或处理失败时返回原图。
- 代理包含 API Key 注入、GraphQL 图片地址重写、独立海报缓存和当前 Emby 网络配置。

插件和配套代理均已实现，尚未替换本机的正式代理与 Emby 插件。
先按 [代理部署说明](proxy/README.md) 启用新代理，再开启插件开关；请求约定见 [代理协议](docs/proxy-contract.md)。
当前开关作用于本插件提供的所有 Scene 主海报，尚不支持按媒体库独立开关。

## 构建与验证

需要 Docker，可访问 Microsoft 镜像仓库和 NuGet。不要求宿主机安装 .NET。

```bash
cd ~/emby_stash
./scripts/build.sh
./scripts/build-proxy.sh
```

脚本使用独立 SDK 容器，先运行 URL 回归测试，再构建 Emby 的 `netstandard2.1` 插件并合并依赖。
不挂载 Docker socket、不连接生产数据库、不调用 Emby 管理 API。NuGet 缓存写入 `.build/`。
可用 `DOTNET_SDK_IMAGE` 指定兼容 .NET 10 的 SDK 镜像或镜像摘要。

产物：

```text
artifacts/emby-stash-auto-poster/Emby.Plugins.Stash.dll
artifacts/Emby.Plugins.Stash-auto-poster.tar.gz
artifacts/SHA256SUMS
artifacts/proxy/stash-poster-proxy
```

代理构建另外生成镜像 `emby-stash-poster-proxy:local`，运行时无需下载人脸模型。

有本地 .NET 10 SDK 时也可以直接运行：

```bash
dotnet run --project tests/PosterImageUrl.Tests/PosterImageUrl.Tests.csproj -c Release
dotnet build Jellyfin.Plugin.Stash/Stash.csproj -c Release.Emby
```

本工程优先验证 Emby 构建；Jellyfin 配置页保持兼容，但不宣称已在 Jellyfin 服务端完成验证。

## 部署顺序

1. 按 [代理部署说明](proxy/README.md) 在独立端口验证后，将现有认证代理切换为配套 Go 代理。
2. 备份当前 Stash 插件 DLL 与配置 XML，停止 Emby，替换原插件 DLL 后启动。
   使用同一个插件 ID，只替换已有 DLL，不要同时安装两个 Stash 插件副本。
3. 在 Stash 插件设置中开启 **Enable automatic poster cropping via proxy**。
4. 选一部已匹配 Stash 的影片重新获取图片，核对主海报、背景图和鉴权；之后再考虑批量更新。
5. 回退时先关闭开关；如需回退二进制，停止 Emby 并恢复原 DLL。已经缓存的裁剪海报需重新获取才会恢复原图。

当前不自动部署、不批量刷新媒体库、不移动视频、不改写 Stash 原图。

## 工程入口

- `Jellyfin.Plugin.Stash/Helpers/PosterImageUrl.cs`：URL 用途标记，保留原查询参数。
- `Jellyfin.Plugin.Stash/Providers/StashAPI.cs`：主海报与背景图分流。
- `Jellyfin.Plugin.Stash/Configuration/PluginConfiguration.cs`：功能开关。
- `tests/PosterImageUrl.Tests/`：无需第三方测试框架的回归用例。
- `docs/proxy-contract.md`：自动人脸裁剪、网络、鉴权与缓存协议。
- `proxy/`：Go 代理、MetaTube 检测源码子集、实际人像回归测试和 Docker 部署配置。

沿用上游仓库 LICENSE 文件中的 MIT 许可，见 [LICENSE](LICENSE)。下面保留上游说明。

---

## Upstream: Jellyfin.Plugin.Stash

This metadata provider helps fill Jellyfin/Emby with information for your adult videos by pulling from your local [Stash](https://stashapp.cc/)

[![GPL 2.0 License](https://img.shields.io/github/license/DirtyRacer1337/Jellyfin.Plugin.Stash)](./LICENSE)
[![Current Release](https://img.shields.io/github/release/DirtyRacer1337/Jellyfin.Plugin.Stash)](https://github.com/DirtyRacer1337/Jellyfin.Plugin.Stash/releases/latest)
[![Build status](https://img.shields.io/github/actions/workflow/status/DirtyRacer1337/Jellyfin.Plugin.Stash/release.yml)](https://github.com/DirtyRacer1337/Jellyfin.Plugin.Stash/releases/tag/latest)

------------

## Install
- Repository (Jellyfin only):
  - Add to the list this URL `https://raw.githubusercontent.com/DirtyRacer1337/Jellyfin.Plugin.Stash/main/manifest.json`
- Manual:
  - Download Archive from [Latest Release](https://github.com/DirtyRacer1337/Jellyfin.Plugin.Stash/releases/latest)
  - Follow the [Instruction](https://jellyfin.org/docs/general/server/plugins/index.html)
