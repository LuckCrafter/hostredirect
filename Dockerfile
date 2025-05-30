FROM --platform=$BUILDPLATFORM golang:1.24.1 AS build

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod ./
COPY go.sum ./

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd ./cmd
COPY plugin.go ./

# Automatically provided by the buildkit
ARG TARGETOS
ARG TARGETARCH

# Build
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -a -o gate cmd/main.go

# Move binary into final image
FROM --platform=$BUILDPLATFORM gcr.io/distroless/static-debian11 AS app
COPY --from=build /workspace/gate /gate
CMD ["/gate"]