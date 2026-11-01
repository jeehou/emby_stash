# GHCR 镜像发布与 Portainer 更新

镜像地址：`ghcr.io/jeehou/emby-stash-poster-proxy`。
`.github/workflows/build.yml` 使用 GitHub Actions 自带的 `GITHUB_TOKEN` 发布，
只给发布 job 设置 `packages: write`，无需把个人访问令牌写进工程。

## 首次启用

1. 提交工作流需要 GitHub 凭据具有 `workflow` 权限；GitHub CLI 可用
   `gh auth refresh -h github.com -s workflow` 在浏览器中补充授权。
2. 将工作流推送到 `main`，等待插件和代理构建全部成功。
3. GitHub 新建的 GHCR 包默认是 Private。打开个人 Packages 中的
   `emby-stash-poster-proxy`，进入 Package settings，将可见性设为 Public。
   仓库公开不代表镜像包自动公开。
4. 在 Docker 主机验证 `docker pull ghcr.io/jeehou/emby-stash-poster-proxy:latest`。
   只有镜像成功发布且拉取验证通过后，再更新 Portainer stack。

以上是首次接入步骤，并不代表镜像已经成功发布；以 GitHub Actions 和实际拉取结果为准。

## 日常更新

- `main` 的检查通过后更新 `latest`，Portainer 更新 stack 时可开启 **Re-pull image**。
- 推送 `v*` Git 标签时发布同名镜像版本，不覆盖 `latest`。
- 每次发布另有 `sha-<12 位提交号>` 标签。需要回退时把 Compose 的镜像标签改回旧提交。
- PR 仅执行构建，不能发布镜像。
- 当前镜像面向 `linux/amd64`；其他架构需另行构建。

工作流保留插件安装包及代理二进制为 Actions artifacts。本地
`./scripts/build-proxy.sh` 仍可用于开发和离线构建。

Portainer stack 中的 Emby 服务、代理名称、监听地址与认证文件挂载不变。
使用 `proxy/compose.portainer.yaml`，无需再设置 `pull_policy: never`。

参考：[GitHub 容器仓库](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)。
