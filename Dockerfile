FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app

COPY src/go.* /app/
RUN go mod download

COPY src/ /app/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/unifi-external-dns ./


FROM --platform=$BUILDPLATFORM buildpack-deps:noble-curl AS chisel

ARG BUILDPLATFORM
ARG CHISEL_VERSION=1.4.0

SHELL ["/bin/bash", "-o", "pipefail", "-c", "-l"]

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install --no-install-recommends -qy \
        file=1:5.45-3build1 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/* /var/tmp/*

RUN case "${BUILDPLATFORM}" in \
        linux/amd64)              ARCH=amd64    ;; \
        linux/arm64 | linux/arm64/v8) ARCH=arm64 ;; \
        linux/arm/v7)             ARCH=armhf    ;; \
        linux/arm/v6)             ARCH=armel    ;; \
        linux/ppc64le)            ARCH=ppc64el  ;; \
        linux/s390x)              ARCH=s390x    ;; \
        linux/386)                ARCH=386      ;; \
        *) echo "Unsupported BUILDPLATFORM: ${BUILDPLATFORM}" >&2; exit 1 ;; \
    esac \
    && curl -fSL --output chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz https://github.com/canonical/chisel/releases/download/v${CHISEL_VERSION}/chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz \
    && curl -fSL --output chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz.sha384 https://github.com/canonical/chisel/releases/download/v${CHISEL_VERSION}/chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz.sha384 \
    && sha384sum -c chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz.sha384 \
    && tar -xzf chisel_v${CHISEL_VERSION}_linux_${ARCH}.tar.gz -C /usr/bin/ chisel \
    && curl -fSL --output /usr/bin/chisel-wrapper https://raw.githubusercontent.com/canonical/rocks-toolbox/v1.2.0/chisel-wrapper \
    && chmod 755 /usr/bin/chisel-wrapper

RUN groupadd \
        --gid=1654 \
        app \
    && useradd -l \
        --uid=1654 \
        --gid=1654 \
        --shell /bin/false \
        app \
    && install -d -m 0755 -o 1654 -g 1654 "/rootfs/home/app" \
    && mkdir -p "/rootfs/etc" \
    && rootOrAppRegex='^\(root\|app\):' \
    && grep "${rootOrAppRegex}" /etc/passwd > "/rootfs/etc/passwd" \
    && grep "${rootOrAppRegex}" /etc/group > "/rootfs/etc/group"


FROM scratch

COPY --from=chisel /rootfs /
COPY --from=build /out/unifi-external-dns /home/app/unifi-external-dns

USER 1654

ENTRYPOINT ["/home/app/unifi-external-dns"]
