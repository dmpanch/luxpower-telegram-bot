A bot for Luxpower inverters that receives checks on the status of the power grid and reports it to Telegram chats when grid is on/off.

Based on projects:
* https://github.com/go-telegram-bot-api/telegram-bot-api
* https://github.com/kgf1980/go-luxpower

The bot takes data from the Luxpower website, where invertor sends updates every 2 minutes.

The current status can be obtained by sending the /status command to the bot.

To run the bot you need to:
* Register Telegram bot and get its token
* Add token and data from Luxpower site to env, rename env to .env
* Build Go binaries for the required architecture (the build could have been put into a Dockerfile, but I didn't care enough)
  * go-luxpower - https://github.com/kgf1980/go-luxpower
    * CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o go-luxpower main.go
  * CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o telegram-bot main.go
* Run docker-compose