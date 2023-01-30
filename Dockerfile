FROM alpine:3.16

RUN apk --no-cache add ca-certificates bash curl coreutils tzdata

WORKDIR /app
COPY MirrorBotGo .
CMD ["./MirrorBotGo"]
