FROM --platform=$BUILDPLATFORM golang:1.22 AS build
ARG TARGETARCH
WORKDIR /usr/src/app
COPY *.go go.mod go.sum ./
COPY internal internal
COPY engine engine
COPY command command
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -tags netgo -ldflags -w -o drone-runner-qemu-$TARGETARCH


FROM alpine AS certs
RUN apk add -U --no-cache ca-certificates


FROM alpine

ARG TARGETARCH

ENV GODEBUG netdns=go
ENV DRONE_PLATFORM_OS linux
ENV DRONE_PLATFORM_ARCH $TARGETARCH

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

LABEL com.centurylinklabs.watchtower.stop-signal="SIGINT"

COPY --from=build /usr/src/app/drone-runner-qemu-$TARGETARCH /bin/drone-runner-qemu
ENTRYPOINT ["/bin/drone-runner-qemu"]
