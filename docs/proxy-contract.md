# 主海报自动裁剪代理协议 v1

本工程包含 Emby/Stash 插件的图片用途标记和 `proxy/` 中的 Go 图片裁剪代理。
`emby_poster=1` 是本工程定义的代理协议，不是 Stash 或 MetaTube 已有的参数。

## 请求

启用 `EnableAutoPosterCrop` 后：

| 图片用途 | 请求示例 |
| --- | --- |
| Scene 主海报、识别结果海报预览 | `/scene/123/screenshot?t=1700&emby_poster=1` |
| Scene 背景图 | `/scene/123/screenshot?t=1700` |
| 演员图片、厂商 Logo | 原地址 |

只在绝对 HTTP(S) URL 上添加标记；不重新编码已有参数值，保留片段。
已有同名标记（含重复、大小写和转义形式）统一成一个 `emby_poster=1`。
开关默认关闭，关闭时 URL 完全保持原样。代理必须保留与图片来源相关的时间戳、鉴权参数。

## 代理处理

1. 配套 Go 代理接替现有认证代理，负责 Stash API Key 和 GraphQL 响应地址重写。
2. 带 `emby_poster=1` 的 screenshot 请求交给自动裁剪处理器；处理器抓取原图时去掉该标记，避免递归处理。
3. 复用 MetaTube 的 `detector.FindPrimaryFaceAxisRatio` 和 `imageutil.CropImagePosition`，目标比例 2:3。
   对 Stash 主海报统一启用人脸检测，而不是固定右裁，也不依赖影片番号触发。
4. 检测采用多角度模式，按人脸尺寸 × 检测得分加权、沿裁剪轴聚类，取总权重最大的组的加权中心。
   对可靠识别出中间书脊的横向封套，额外限制到主脸所在的封面区域；其他图片继续人脸居中。
5. 检测无结果或处理异常时返回原图，当前没有单片位置覆盖功能。这是本工程的回退策略，
   不等同于 MetaTube 默认影片流程的右侧回退。
6. 没有标记的请求返回原图。不得覆盖 Stash 原始图片。
7. 原图与海报的缓存必须区分；缓存键包含原始图片版本参数、用途、比例和算法版本。

标记不保证能够完整保留人物身体：MetaTube 提供的是人脸引导裁剪，而非全身分割。

## 现有部署对接注意

当前 `stash-auth-proxy` 与 Emby 共享网络命名空间，仅监听 `127.0.0.1:18080`。
独立裁剪容器不能用自己的 `127.0.0.1:18080` 访问它：需要安排共享网络命名空间，
或通过私有网络访问 Stash，并在服务端安全提供认证信息。不要把 Stash API Key 添加到海报标记中。

单独给 Nginx 增加查询参数匹配不具备图像检测能力。配套 Go 代理已实现本协议，
但运行中的旧 Nginx 代理尚未切换；部署说明见 [proxy/README.md](../proxy/README.md)。

## 集成验收

- 同一 scene：Primary URL 有且仅有一个标记，Backdrop URL 无新增标记。
- 左、中、右侧主脸的封面分别以检测到的主要区域为中心，输出 2:3。
- 多人／拼接封面按照检测组总权重选择位置；无脸图按回退策略返回。
- 主海报和背景图缓存互不污染；版本参数变化立即生成新缓存键，同 URL 原图变更最长等待海报缓存 TTL（默认 5 分钟）。
- 关闭功能并重新下载图片，可恢复原图。

## 源码依据

- https://github.com/metatube-community/metatube-sdk-go/blob/main/detector/detector.go
- https://github.com/metatube-community/metatube-sdk-go/blob/main/detector/compute.go
- https://github.com/metatube-community/metatube-sdk-go/blob/main/engine/image.go
- https://github.com/metatube-community/metatube-sdk-go/blob/main/imageutil/crop.go

本工程固定使用 MetaTube 提交 `fdadc9c8e321947fb6533fb0beab2a92f1c2b065` 的源码子集，
已保留 Apache-2.0 许可证和原文件校验和，见 `proxy/third_party/metatube/PROVENANCE.md`。
