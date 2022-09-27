FROM golang:alpine AS builder

# Working directory
WORKDIR /app
COPY . .
RUN apk --no-cache add ca-certificates curl gcc musl-dev g++ bash coreutils
RUN go build -ldflags "-s -w" -tags disable_mega -o bot
# Command to run when starting the container

FROM alpine:latest
RUN apk --no-cache add ca-certificates curl gcc musl-dev g++ bash coreutils
WORKDIR /app
COPY --from=builder /app/bot /app/bot
RUN mkdir downloads
COPY build .
# ENTRYPOINT /app
CMD ["/app/bot"]