FROM alpine:3.16

RUN apk --no-cache add ca-certificates bash curl coreutils tzdata

WORKDIR /app
RUN export ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
        export ARCH="amd64"; \
    fi && \
    if [ "$ARCH" = "aarch64" ]; then \
        export ARCH="arm64"; \
    fi && \
    wget -O /usr/bin/megasdkrest https://github.com/ViswanathBalusu/megasdkrest/releases/latest/download/megasdkrest-$ARCH && \
    chmod +x /usr/bin/megasdkrest
COPY MirrorBotGo .
VOLUME [ "/app" ]
CMD ["./MirrorBotGo"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl --fail http://localhost:7870/health || exit 1