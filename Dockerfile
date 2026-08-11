# syntax=docker/dockerfile:1.7
FROM golang:1.26.5@sha256:7caba5286b4c3613a337b709c573047d8ae62ee76106647313b61e72b99f20af AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /scheduled-woop ./cmd/scheduled-woop

FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
LABEL org.opencontainers.image.source="https://github.com/ronakforcast/scheduled-woop"
LABEL org.opencontainers.image.description="Schedule CAST AI Workload Autoscaler policy settings"
LABEL org.opencontainers.image.licenses="MIT"
COPY --from=builder /scheduled-woop /scheduled-woop
USER 65532:65532
ENTRYPOINT ["/scheduled-woop"]
