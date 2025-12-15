# syntax=docker/dockerfile:1

ARG GO_VERSION=1.23.0

FROM almalinux:9 AS lbctl-test

ARG GO_VERSION
ARG TARGETARCH

RUN dnf -y update && \
    dnf -y install ca-certificates curl-minimal tar gzip git findutils make && \
    dnf -y clean all

# Install Go toolchain (AlmaLinux repos may not have Go 1.23 yet; go.mod requires it).
RUN set -euo pipefail; \
    arch="${TARGETARCH:-amd64}"; \
    case "$arch" in \
      amd64) goarch="amd64" ;; \
      arm64) goarch="arm64" ;; \
      *) echo "Unsupported TARGETARCH: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${goarch}.tar.gz" -o /tmp/go.tgz; \
    rm -rf /usr/local/go; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm -f /tmp/go.tgz

ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

FROM lbctl-test AS lbctl-build
RUN make build
RUN install -D -m 0755 "bin/lbctl" "/out/lbctl"

FROM almalinux:9 AS lbctl-runtime
RUN dnf -y update && \
    dnf -y install ca-certificates && \
    dnf -y clean all
COPY --from=lbctl-build /out/lbctl /usr/local/bin/lbctl
ENTRYPOINT ["lbctl"]
CMD ["--help"]
