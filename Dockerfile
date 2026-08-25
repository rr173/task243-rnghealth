FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/rnghealth ./cmd/rnghealth

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app
COPY --from=builder /out/rnghealth /app/rnghealth
ENTRYPOINT ["/app/rnghealth"]
CMD ["--smoke-test"]
