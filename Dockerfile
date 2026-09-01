FROM golang:1.26-alpine AS deps
WORKDIR /deps
RUN apk add --no-cache git
RUN git clone --depth 1 https://git.zem.systems/muxcore/core.git core && \
    git clone --depth 1 https://git.zem.systems/muxcore/contracts-media.git contracts-media

FROM golang:1.26-alpine AS builder
WORKDIR /workspace/database-postgres
COPY --from=deps /deps/core /workspace/core
COPY --from=deps /deps/contracts-media /workspace/contracts-media
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /workspace/database-postgres/module ./cmd/module

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /workspace/database-postgres/module /
EXPOSE 9701
ENTRYPOINT ["/module"]
