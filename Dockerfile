# syntax=docker/dockerfile:1.7
FROM golang:1.26.2 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /scheduled-woop ./cmd/scheduled-woop

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /scheduled-woop /scheduled-woop
USER 65532:65532
ENTRYPOINT ["/scheduled-woop"]
