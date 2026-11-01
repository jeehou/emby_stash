# Stash 自动海报代理

Go 实现的认证反向代理，可接替现有 `stash-auth-proxy`。使用本工程插件发出的
`emby_poster=1` 区分主海报与背景图，只有 Scene 主海报进入自动人脸检测和 2:3 裁剪。

## 实际处理链

```text
Emby Stash 插件
  ├─ GraphQL → 代理注入 ApiKey → Stash → 响应图片 URL 重写回代理
  ├─ screenshot?emby_poster=1 → 取原图 → MetaTube 多角度人脸检测 → 2:3 JPEG
  └─ screenshot（无标记） → Stash 原图
```

原样复用 MetaTube `fdadc9c8e321947fb6533fb0beab2a92f1c2b065` 的检测与裁剪源码和
Pigo 模型，放在 `third_party/metatube`，附源码校验和及 Apache-2.0 许可。
只引入所需子集，不引入 MetaTube 数据库、在线刮削器或外部 AI 服务。

- 多角度检测人脸，以尺寸 × 检测得分计算权重，沿裁剪轴聚类。
- 选择总权重最大的组，以加权中心裁成 2:3，JPEG 质量 90。
- **不固定右裁**，也不按番号限制检测；无法检测到人脸时返回原始字节。
- 解码异常、像素超过 2400 万、任一边超过 12000 像素、文件超过 16 MiB、工作槽已满时回退原图。
- 直接支持 JPEG、PNG、GIF 首帧及 WebP；不支持的格式回退原图。
- 认证由服务端注入，不向 URL 追加密钥。抓图不跟随外部重定向，以免把密钥转发到其他站点。
- 只处理 `/scene/<数字 ID>/screenshot`，演员、Logo 和其他路径不裁剪。
- 不记录完整请求 URL、查询参数或认证值。
- 只做“人脸引导构图”，不保证保留完整身体。大尺寸图片回退时不强制 2:3。

`X-Emby-Poster` 响应头说明本次处理结果：

| 值 | 含义 |
| --- | --- |
| `auto` | 自动定位并裁剪成功；`X-Emby-Poster-Position` 给出归一化中心 |
| `original-no-face` | 没检测到人脸，保留原图 |
| `original-error` | 解码／检测异常或像素限制，保留原图 |
| `original-too-large` | 超过 16 MiB，流式返回原图 |
| `original-busy` | 处理槽已满，原样转发 |

上游 HTTP 错误保留原状态码；网络连接失败返回 502，不伪造成功图片。

## 构建与测试

从工程根目录：

```bash
./scripts/build-proxy.sh
```

生成镜像 `emby-stash-poster-proxy:local`，并导出二进制和许可证到 `artifacts/proxy/`。
Docker 构建会校验 vendored 源码并运行测试。镜像默认非 root、无需写入持久数据；
运行时不需要下载人脸模型或访问互联网。

本机有 Go 1.24 或更高版本时：

```bash
cd proxy
go test -race ./...
go vet ./...
go build -trimpath -o ../artifacts/proxy/stash-poster-proxy .
```

测试包含 NASA 公共领域人像的左／中／右位置、多人权重、90°旋转、无脸回退，
以及请求鉴权、JSON URL 重写、缓存隔离、时间戳更新、HEAD/ETag、外部重定向和忙碌回退。

## 当前 Emby 的配套部署

`compose.emby.yaml` 为当前部署准备：共享 `emby` 网络命名空间，仅监听
`127.0.0.1:18080`，只读挂载原有 Stash 插件 XML 获取 API Key，上游为 `10.0.0.8:9999`。
这样现有插件中的代理地址可以继续使用，不需要公开额外 LAN 端口。

它是 **现有 Nginx 代理的替代服务**，不是和旧代理同时占用 18080 的第二个服务。
本工程的构建、测试脚本不会自动切换生产服务。

部署时按顺序操作：

1. 先完成镜像构建，并备份现有插件 DLL、配置 XML 和旧代理配置。
2. 用独立端口测试新代理；确认 Stash GraphQL、原图和自动海报都可访问。
3. 切换代理时停止旧的 `stash-auth-proxy`，再运行：

   ```bash
   cd ~/emby_stash/proxy
   docker compose -p emby-stash-poster -f compose.emby.yaml up -d
   ```

4. 安装配套 DLL，开启 `EnableAutoPosterCrop`，仅选一个条目重新下载图片验证。
   旧缓存不会自动改成裁剪海报。
5. 回退时先关闭插件开关，停止新代理，再启动原有 `stash-auth-proxy`。

如 Emby 容器被重新创建（网络命名空间改变），应重新创建新代理容器，让它重新加入当前
Emby 的网络命名空间。XML 必须能由容器用户 65532 读取；不要为此把 API Key 文件公开。
配套配置添加了当前 Docker 内配置文件所属组 `998`，实测可读。换环境时用
`docker exec stash-auth-proxy stat -c %g /run/secrets/emby-stash.xml` 查看组号，
通过 `STASH_CONFIG_GID` 覆盖；宿主机与容器显示的组号可能不同。
文件中 API Key 发生变更后需要重启代理。

## 通用独立部署

`compose.yaml` 默认只向宿主机回环地址发布 18081。复制 `.env.example` 为 `.env`，
填写真实 `STASH_UPSTREAM` 和 **Emby 能访问的** `PROXY_PUBLIC_URL`，把密钥单独放入
`secrets/stash_api_key`，再运行 `docker compose up -d --build`。

独立 Docker 容器内的 `127.0.0.1` 不是宿主机或 Emby 容器。对于当前环境，应优先使用
上面的 `compose.emby.yaml`。本代理自身没有客户端登录功能，应维持回环或受控私有网络访问。

## 配置

| 环境变量 | 默认值／说明 |
| --- | --- |
| `STASH_UPSTREAM` | 必填，例如 `http://10.0.0.8:9999`，只接受 origin |
| `PROXY_PUBLIC_URL` | 必填，重写到 Emby 可达的代理 origin |
| `LISTEN_ADDR` | `127.0.0.1:18080` |
| `STASH_API_KEY_FILE` | 纯文本密钥文件，优先级最高 |
| `EMBY_STASH_CONFIG_FILE` | Emby Stash XML，读取 `StashAPIKey`，次优先 |
| `STASH_API_KEY` | 环境变量密钥，最后选择 |
| `STASH_REWRITE_ORIGINS` | 逗号分隔的额外 Stash 旧地址；当前上游自动包括在内 |
| `CACHE_MAX_MB` | 内存 LRU 缓存大小，默认 64 MiB，0 关闭 |
| `CACHE_TTL` | 默认 `5m`；相同 URL 内容变化但无版本参数时最长延迟 |
| `CROP_WORKERS` | 默认 2，范围 1–16；一个任务内部仍有多角度并行 |
| `UPSTREAM_TIMEOUT` | 默认 `20s`；抓图总请求和转发响应头超时 |

缓存键包含完整源路径及查询参数、固定算法版本（含比例和编码质量）；海报缓存独立于原图。
`t=` 等版本参数变化会产生新缓存键；无版本参数时依赖 TTL，重启可清空缓存。
遵守上游 `no-store`/`no-cache`，不缓存带 `Set-Cookie` 的图片；浏览器响应为 private。
原图反向代理保留 Stash 的缓存行为。

图片处理只在内存发生，不把用户图片落盘。`/healthz` 表示代理进程可用，
不代表上游 Stash 已通过连通性检查。
