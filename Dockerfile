ARG GO_IMAGE=docker.m.daocloud.io/library/golang:1.26-alpine
ARG GOST_IMAGE=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/gogost/gost:3.2.6
ARG RUNTIME_IMAGE=docker.m.daocloud.io/library/alpine:latest

FROM ${GO_IMAGE} AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o proxy-runtime ./cmd/proxy-runtime

FROM ${GOST_IMAGE} AS gost

FROM ${RUNTIME_IMAGE}

WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=gost /bin/gost /usr/local/bin/gost
COPY --from=builder /app/proxy-runtime /usr/local/bin/proxy-runtime

EXPOSE 8080

CMD ["proxy-runtime"]
