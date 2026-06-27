# 多阶段构建：node 构建前端 → golang 编译（embed 前端）→ distroless 单二进制。

# 1) 构建前端
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# 2) 编译 Go（CGO_ENABLED=0 纯静态，embed 前端产物）
# 用 vendor/ 离线构建，避免容器内联网拉模块。
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -mod=vendor -o /k8smanage ./cmd/server

# 3) 运行镜像：distroless 静态（无 shell、无包管理器，攻击面小）
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="k8smanage" \
      org.opencontainers.image.description="面向 k3s 小集群的管理+监控一体工具（单镜像）" \
      org.opencontainers.image.source="https://github.com/sunhao/k8smanage"
COPY --from=build /k8smanage /k8smanage
EXPOSE 8080
ENTRYPOINT ["/k8smanage"]
