FROM alpine:latest
RUN apk --no-cache add ca-certificates

COPY ./go-luxpower /app/go-luxpower
COPY ./telegram-bot /app/telegram-bot

WORKDIR /app

RUN chmod +x go-luxpower telegram-bot

ENTRYPOINT ["./telegram-bot"]