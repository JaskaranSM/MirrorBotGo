FROM ubuntu:22.04

RUN apt-get update && apt-get install ca-certificates bash curl coreutils tzdata locales -y

WORKDIR /app

RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8

COPY MirrorBotGo .
CMD ["./MirrorBotGo"]
