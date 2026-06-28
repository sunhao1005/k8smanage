# k8smanage

面向 **k3s 小集群（单机 4 核 8G 级别）** 的「集群管理 + 基础监控」一体工具。

设计目标：**高性能、最大化复用开源、单镜像部署**。整套服务打包成一个 Go 二进制（前端 embed 进去），复用 k3s 自带的 metrics-server，新增常驻内存约 120MB。

> 状态：**M1–M5 全部完成，已在真 k3s（单节点 Docker `rancher/k3s`）上端到端验证**。完整计划见 [docs/superpowers/plans/2026-06-27-k8smanage.md](docs/superpowers/plans/2026-06-27-k8smanage.md)。
>
> 公开镜像：**`docker pull 17719317036/k8smanage:v0.4.0`**（[Docker Hub](https://hub.docker.com/r/17719317036/k8smanage)）

## 功能一览

- **监控**：节点 CPU/内存/磁盘/网络/负载 + Pod CPU/内存，历史曲线（环形存储），Web 控制台总览大盘。
- **管理**：工作负载扩缩容、滚动重启、**暂停/启用**（缩到 0 / 恢复原副本数）、删 Pod，实时日志流，浏览器内容器终端（TTY）。
- **告警**：阈值规则（持续时长、匹配单个或全部目标）、状态机（pending→firing→恢复）、webhook 通知（飞书/钉钉/通用 JSON）。
- **控制台**：自建 React SPA（总览 / 工作负载 / 日志 / 终端 / 告警），embed 进同一个二进制。
- **部署**：单个 distroless 镜像（约 12.7MB）+ 一条 `kubectl apply`。

## 为什么不用现成的重型栈

小集群上 Prometheus + Grafana + 一堆采集器起步就要 0.5–1GB 内存，对单机 k3s 偏重。本工具把它压成：

| 能力 | 方案 |
|---|---|
| 节点指标（CPU/内存/磁盘/网络/负载） | 进程内用 [gopsutil](https://github.com/shirou/gopsutil) 直接读宿主 `/proc`，不另起采集器 |
| Pod 指标（CPU/内存） | 复用 k3s 自带 **metrics-server**（读 `metrics.k8s.io`） |
| 对象状态（节点/工作负载/事件） | client-go + controller-runtime **informer 本地缓存**，读不打 apiserver |
| 时序存储 | 进程内嵌 [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)（纯 Go 无 CGO），环形保留 |
| 可视化 + 管理 | 自建 React 控制台，embed 进同一个二进制 |

存储与节点采集都走接口（`MetricStore` / `NodeCollector`），将来要换 VictoriaMetrics 或扩到多节点，前端和 API 不动。

## 架构

```
浏览器 ── REST/WebSocket ──▶ 单个 Go 进程
                              ├─ informer 缓存   → k8s 对象状态（读不打 apiserver）
                              ├─ gopsutil        → 宿主机节点指标
                              ├─ metrics-server  → Pod 指标
                              ├─ SQLite 环形存储  → 历史样本（落可写卷）
                              └─ chi REST/WS + embed 前端
```

## 目录结构

```
cmd/server/        进程入口（装配 config→store→采集→告警→HTTP）
internal/
  config/          环境变量配置
  metrics/         采集与存储：MetricStore(SQLite) / NodeCollector(gopsutil) / Pod 指标 / 采集循环
  kube/            k8s 客户端 / informer 缓存 / 对象读取(lister) / 写操作(actions) / 日志·exec 流(stream)
  alert/           告警：规则存储 / 评估状态机 / webhook 通知
  api/             鉴权中间件 + REST/WS API + 静态资源(SPA 回退)
web/               前端 React+Vite（src/ 源码，dist/ 构建产物被 embed）
deploy/            Dockerfile + k8s 部署清单
vendor/            依赖 vendor（-mod=vendor，容器离线构建）
docs/              实现计划
```

## 构建

需要 Go 1.26+（go.mod 要求 1.26）。依赖已 vendor，离线即可编译。前端构建产物会 embed，纯后端开发可用占位前端直接编：

```bash
CGO_ENABLED=0 go build -o k8sm-server ./cmd/server
```

完整构建（含真实前端）：

```bash
cd web && npm install && npm run build && cd ..   # 生成 web/dist
CGO_ENABLED=0 go build -o k8sm-server ./cmd/server
```

前端开发（热更新，自动把 /api 代理到本地后端）：

```bash
cd web && npm run dev   # http://localhost:5173
```

构建单镜像：

```bash
docker build -f deploy/Dockerfile -t k8smanage:latest .
```

> 也可直接用已发布的预构建镜像（无需自己构建）：
> **`docker pull 17719317036/k8smanage:v0.4.0`**（[Docker Hub](https://hub.docker.com/r/17719317036/k8smanage)，公开）。

## 本地运行

**纯本机模式**（不连集群，只采本机指标，用于快速试跑）：

```bash
K8SM_DISABLE_K8S=1 K8SM_DB_PATH=./k8sm.db go run ./cmd/server
# 打开 http://localhost:8080/api/overview
```

**连接 k3s**（设置 kubeconfig 即自动连接，失败则降级为纯本机模式）：

```bash
KUBECONFIG=/path/to/k3s.yaml K8SM_DB_PATH=./k8sm.db go run ./cmd/server
```

## 部署到 k3s

```bash
# 1) 准备镜像，二选一：
#  a) 直接用已发布的公开镜像（推荐，最省事）：
#     把 deploy/k8smanage.yaml 里的 image 改成 17719317036/k8smanage:v0.4.0 即可，k3s 会自动拉取
#  b) 或本地构建并导入 k3s（无镜像仓库时）：
docker build -f deploy/Dockerfile -t k8smanage:latest .
docker save k8smanage:latest | sudo k3s ctr images import -

# 2)（建议）创建登录账号密码
kubectl -n kube-system create secret generic k8smanage-auth \
  --from-literal=user=admin \
  --from-literal=pass='你的密码' \
  --from-literal=session-secret=$(openssl rand -hex 16)

# 3) 部署
kubectl apply -f deploy/k8smanage.yaml

# 4) 访问
kubectl -n kube-system port-forward svc/k8smanage 8080:80
# 打开 http://localhost:8080 ，用上面的账号密码登录
```

清单要点：最小 RBAC、不开 `hostPID`、host `/proc`+`/sys`+`/` 只读挂载、SQLite 落 hostPath 持久化、只读根文件系统 + `/tmp` emptyDir、liveness/readiness 探针。多节点场景见下方说明。

## 用 docker compose 跑（不进 k8s，单机最省事）

单机 Linux 上不想走 k8s 部署，可直接用 docker 跑，连本机 k3s：

```bash
K8SM_AUTH_PASS='你的密码' docker compose up -d --build
# 浏览器打开 http://<服务器IP>:8080 ，用 admin / 你的密码 登录
```

> 想跳过本地构建，把 [docker-compose.yml](docker-compose.yml) 里的 `build:` 删掉、`image:` 改成 `17719317036/k8smanage:v0.4.0`，直接拉公开镜像跑。

[docker-compose.yml](docker-compose.yml) 用 `network_mode: host` 直连本机 k3s 的 `127.0.0.1:6443`，挂 `/etc/rancher/k3s/k3s.yaml` 作 kubeconfig、挂宿主 `/proc`·`/sys`·`/` 采集指标、SQLite 落到 `./data/`。仅适用于 Linux 主机。

## 域名子路径反代

想用 `https://你的域名/k8smanage/` 这种子路径访问（而非独立域名），设 `K8SM_BASE_PATH=/k8smanage` 即可——后端会把整个应用挂到该前缀下，并往首页注入 `<base href>` 与运行时变量，前端的资源、API、WebSocket 全部自动带前缀。**健康检查 `/healthz` 始终在根，不受影响**（探针无需改）。一份镜像适配任意子路径，无需重新构建。

反代需**保留路径前缀**（不要 rewrite 掉）。

nginx：

```nginx
location /k8smanage/ {
    proxy_pass http://127.0.0.1:8080;   # 末尾不带斜杠 = 保留完整 /k8smanage/ 前缀
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Upgrade $http_upgrade;     # WebSocket（日志/终端）
    proxy_set_header Connection "upgrade";
}
```

k8s Ingress（nginx-ingress，默认保留路径、不加 rewrite-target）：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: { name: k8smanage, namespace: kube-system }
spec:
  rules:
    - host: 你的域名
      http:
        paths:
          - path: /k8smanage
            pathType: Prefix
            backend: { service: { name: k8smanage, port: { number: 80 } } }
```

同时给 Deployment 设 `K8SM_BASE_PATH=/k8smanage`（manifest env 里加一条即可）。

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `K8SM_ADDR` | `:8080` | HTTP 监听地址 |
| `K8SM_INTERVAL_SEC` | `15` | 采集间隔（秒） |
| `K8SM_RETENTION_SEC` | `604800` | 样本保留时长（默认 7 天） |
| `K8SM_DB_PATH` | `/data/k8smanage.db` | SQLite 路径，须落在**可写卷**上 |
| `K8SM_HOST_ROOT` | `/` | 宿主根分区在容器内的挂载点（磁盘采集用） |
| `K8SM_AUTH_USER` | 空 | 登录用户名（与 `K8SM_AUTH_PASS` 同时设置即启用账号密码登录） |
| `K8SM_AUTH_PASS` | 空 | 登录密码（放 k8s Secret） |
| `K8SM_SESSION_SECRET` | 空 | 会话令牌签名密钥；设了则**重启不掉登录态**，留空则随机生成 |
| `K8SM_SESSION_TTL_HOURS` | `24` | 登录会话有效期（小时） |
| `K8SM_AUTH_TOKEN` | 空 | 可选静态 API Token（供脚本调用，与账密登录并存） |
| `K8SM_ALERT_WEBHOOK` | 空 | 告警通知地址（M4） |
| `K8SM_BASE_PATH` | 空 | 子路径前缀（如 `/k8smanage`）；空=根路径。用于域名子路径反代，见下 |
| `K8SM_DISABLE_K8S` | 空 | 设为 `1` 则不连集群（纯本机指标） |
| `NODE_NAME` | 主机名 | 节点标识，部署时用 downward API 注入 |

## API

鉴权：配置了账号密码（`K8SM_AUTH_USER`/`K8SM_AUTH_PASS`）后，前端显示登录页；`POST /api/login {username,password}` 返回会话令牌，请求带 `Authorization: Bearer <token>`（WebSocket 用 `?token=<token>`）。也可用静态 `K8SM_AUTH_TOKEN` 直接作为令牌（脚本用）。`GET /api/config`、`POST /api/login`、`/healthz` 不鉴权。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/config` | 鉴权配置（`authEnabled`/`loginEnabled`），不鉴权 |
| POST | `/api/login` | 账号密码登录 `{username,password}` → `{token}`，不鉴权 |
| GET | `/api/overview` | 节点 + 工作负载汇总 + 每节点最新水位 |
| GET | `/api/nodes` | 节点列表（就绪态 / 角色 / kubelet 版本） |
| GET | `/api/workloads?ns=` | 工作负载列表（Deployment/StatefulSet/DaemonSet，含副本就绪数） |
| GET | `/api/metrics/query?kind=&target=&metric=&from=&to=&step=` | 时序点；`metric ∈ {cpu,mem,disk,net_rx,net_tx,load1}`（`net_rx`/`net_tx` 返回**速率 字节/秒**，已从累计计数器换算并处理重置），时间为 unix 秒、可省（默认近 1 小时）；区间过长时**自动按桶降采样**（≤500 点），可用 `step`（秒）显式指定桶大小 |
| POST | `/api/workloads/{ns}/{kind}/{name}/scale` | 扩缩容，body `{"replicas":N}` |
| POST | `/api/workloads/{ns}/{kind}/{name}/restart` | 滚动重启（等价 rollout restart） |
| POST | `/api/workloads/{ns}/{kind}/{name}/pause` | 暂停：记下原副本数到注解并缩到 0（仅 Deployment/StatefulSet） |
| POST | `/api/workloads/{ns}/{kind}/{name}/resume` | 启用：从注解恢复原副本数（缺省 1） |
| DELETE | `/api/pods/{ns}/{name}` | 删除 Pod（由控制器重建） |
| GET(WS) | `/api/logs?ns=&pod=&container=&follow=1` | 流式 Pod 日志 |
| GET(WS) | `/api/exec?ns=&pod=&container=&shell=/bin/sh` | 容器内终端（TTY，支持 resize） |
| GET | `/api/alerts/rules` | 告警规则列表 |
| PUT | `/api/alerts/rules` | 新增/更新规则（body 为规则 JSON，无 id 自动生成） |
| DELETE | `/api/alerts/rules/{id}` | 删除规则 |
| GET | `/api/alerts/active` | 当前 pending/firing 的告警 |

> 写操作 / 流式在未连接集群时返回 503。WebSocket 鉴权用查询参数 `?token=`。
>
> 告警规则示例（节点内存使用率持续 60s 超 85% 即触发）：
> `{"name":"内存高","kind":"node","target":"","metric":"mem","cmp":">","threshold":0.85,"forSec":60,"enabled":true}`
> （`metric` 中 `mem`/`disk` 为使用率 0..1，`cpu` 为核数，`load1` 为负载；`target` 空=匹配该类全部）。配置 `K8SM_ALERT_WEBHOOK` 后触发/恢复以 JSON POST 通知。

## 测试

```bash
go test ./...
```

单元测试用 fake k8s client / 内存 SQLite，不依赖真实集群。集成验证用 Docker 起单节点 k3s（`rancher/k3s` 镜像）连真集群跑通。

## 安全提示

- 本工具默认不鉴权，**务必设置 `K8SM_AUTH_USER`/`K8SM_AUTH_PASS` 开启登录**，且只用 `ClusterIP` 暴露、别裸开 NodePort/LoadBalancer。
- 读宿主指标需把宿主 `/proc`、`/sys` 只读挂进容器（部署清单已给出，用挂载 + `HOST_PROC`/`HOST_SYS` 而非 `hostPID`，降低权限）。
- 凭据（kubeconfig、token、webhook 密钥）只从环境变量或挂载读，不入库、不进镜像层。

## 多节点

当前单进程只采集**自己所在节点**的主机指标（gopsutil 读本机 `/proc`），适合单机 k3s。扩到多节点时：Pod 指标走 metrics-server 本就是跨节点的、无需改；节点主机指标需为每个节点加一个轻量 agent（DaemonSet）把数据推给主服务——`NodeCollector` 接口已为此预留，API 与前端不动。

## 路线图

- [x] M1 采集 + SQLite 存储
- [x] M2 连 k8s（informer 缓存）+ Pod 指标 + 只读聚合 API + 鉴权
- [x] M3 管理操作（扩缩 / 重启 / 删 Pod / 日志流 / Web 终端）
- [x] M4 阈值告警 + webhook 通知
- [x] M5 React 控制台 + 单镜像打包 + 一键部署清单
