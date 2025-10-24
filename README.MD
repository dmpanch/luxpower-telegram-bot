A bot for Luxpower inverters that receives checks on the status of the power grid and reports it to Telegram chats when grid is on/off.

Based on projects:
* https://github.com/go-telegram-bot-api/telegram-bot-api
* https://github.com/kgf1980/go-luxpower

The bot takes data from the Luxpower website, where invertor sends updates every 2 minutes.

The current status can be obtained by sending the /status command to the bot.

## Configuration

Copy `env` to `.env` and fill the variables:

* `TELEGRAM_BOT_TOKEN` — Telegram bot token.
* `LUXPOWER_ACCOUNT`, `LUXPOWER_PASSWORD`, `LUXPOWER_STATION`, `LUXPOWER_BASEURL` — credentials for Luxpower.
* `SUBSCRIBERS_DB_PATH` — optional path to the SQLite file for storing chat subscriptions (defaults to `subscribers.db` in the working directory).
* `LUXPOWER_PROXY_URL` — optional HTTP(S) proxy for `go-luxpower` requests (leave empty to call the API directly).

To run the bot you need to:
* Register Telegram bot and get its token
* Add token and data from Luxpower site to `.env`
* Install Go dependencies
  * `go mod tidy`
* Build Go binaries for the required architecture (the build could have been put into a Dockerfile, but I didn't care enough)
  * go-luxpower - https://github.com/kgf1980/go-luxpower
    * `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o go-luxpower main.go`
  * `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o telegram-bot main.go`
* Run docker-compose

To validate the build, run `go test ./...`.
