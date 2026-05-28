ARG GO_IMAGE=docker.m.daocloud.io/library/golang:1.26-alpine
ARG GOST_IMAGE=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/gogost/gost:3.2.6
ARG MIHOMO_IMAGE=docker.io/metacubex/mihomo:v1.19.25
ARG RUNTIME_IMAGE=docker.m.daocloud.io/library/alpine:latest


FROM docker.m.daocloud.io/library/node:22-bookworm-slim AS dashboard_remote_builder

WORKDIR /proxy-runtime/webui
COPY common-lib/ui /common-lib/ui
COPY common-lib/proto /common-lib/proto
COPY common-lib/scripts /common-lib/scripts
COPY proxy-runtime/webui ./
RUN npm ci && npm run build

FROM ${GO_IMAGE} AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY common-lib ./common-lib
COPY proxy-runtime/go.mod proxy-runtime/go.sum ./proxy-runtime/
WORKDIR /app/proxy-runtime
RUN go mod download

COPY proxy-runtime ./
RUN go build -o proxy-runtime ./cmd/proxy-runtime

FROM ${GOST_IMAGE} AS gost

FROM ${MIHOMO_IMAGE} AS mihomo

FROM ${RUNTIME_IMAGE} AS mihomo_extract
COPY --from=mihomo / /mihomo-root
RUN set -eux; \
    for candidate in /mihomo-root/mihomo /mihomo-root/usr/bin/mihomo /mihomo-root/usr/local/bin/mihomo /mihomo-root/bin/mihomo; do \
      if [ -f "$candidate" ]; then cp "$candidate" /mihomo; chmod +x /mihomo; exit 0; fi; \
    done; \
    found="$(find /mihomo-root -type f -name mihomo | head -n 1)"; \
    test -n "$found"; \
    cp "$found" /mihomo; chmod +x /mihomo

FROM ${RUNTIME_IMAGE}

WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=gost /bin/gost /usr/local/bin/gost
COPY --from=mihomo_extract /mihomo /usr/local/bin/mihomo
COPY --from=builder /app/proxy-runtime/proxy-runtime /usr/local/bin/proxy-runtime
COPY --from=dashboard_remote_builder /proxy-runtime/webui/dist /app/dashboard/proxy-runtime

EXPOSE 8080

CMD ["proxy-runtime"]
