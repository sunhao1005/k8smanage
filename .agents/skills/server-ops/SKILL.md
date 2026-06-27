---
name: server-ops
description: 操作服务器、SSH、Docker、Kubernetes，或做部署运维时使用。讲高效且安全的操作套路（安全底线见 AGENTS.md，本技能是效率与套路）。
---

# 服务器 / 容器 / 编排操作

> 破坏性/生产操作的安全底线见 AGENTS.md「安全底线」；本技能讲效率与套路。

## 通用（SSH / Shell）
- **复用连接**：跳板/常用主机写进 `~/.ssh/config` + ControlMaster，免重复输入。
- **重复/批量操作脚本化、幂等化**：先单台或 `--dry-run` 验证，再铺开。
- **长任务防断连**：用 `tmux`/`screen`/`nohup`，输出留日志。
- **先查后改**：先用只读命令确认现状，再执行变更。
- **杀进程/清端口要精确**：按端口 `fuser -k <port>/tcp` 或精确 PID，别用宽泛 `pkill -f` 误杀脚本自身。

## Docker
- 确认 context（`docker context ls` / `$DOCKER_HOST`），别本地远程搞混。
- tag 别只用 `latest`，用版本/commit tag 便于回滚；推送前确认**仓库 + 命名空间**。
- **凭据不打进镜像**（`ARG`/`ENV` 留历史层），运行时注入；`.dockerignore` 排密钥。
- 多阶段构建减体积、利用层缓存，必要时配国内代理加速。

## K8s
- 确认 context + namespace（`kubectl config current-context`），生产慎重；变更前 `diff` / `--dry-run=client` 预览。
- **优先声明式**（`apply -f|-k` / Helm），手改要回写清单，保证 git 与集群一致。
- 更新走 `rollout restart` + `rollout status`，出错 `rollout undo`；别「随手 `delete pod` 求重启」。
- **`delete` 前反复确认 namespace + label selector**，选错会一次误删一片。
- Secret 不进 git；改 ConfigMap/Secret 多数需 `rollout restart` 才生效。
- 排障套路：`get pods` → `describe`（看 Events）→ `logs -p`（上一个崩溃容器）→ `exec`；先确认 `KUBECONFIG` 指对集群。
