FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY src/go.* /app/
RUN go mod download

COPY src/ /app/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/unifi-external-dns ./

FROM alpine:3.23.3

RUN adduser -D -g '' app
COPY --from=build /out/unifi-external-dns /usr/local/bin/unifi-external-dns
USER app

ENTRYPOINT ["/usr/local/bin/unifi-external-dns"]
