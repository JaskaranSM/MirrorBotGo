FROM alpine:3.16

RUN apk --no-cache add ca-certificates bash curl coreutils

WORKDIR /app
COPY MirrorBotGo .
VOLUME [ "/app" ]
CMD ["./MirrorBotGo"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl --fail http://localhost:7870/health || exit 1