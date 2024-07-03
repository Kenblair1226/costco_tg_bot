## Costco price checking bot for telegram

A bot that can check Taiwan costco online store price. This bot will take keywords as input and crawl costco website every 10 minutes for price information. If new products found or found an existing product has a price drop, send notification to your channel.

### Requirements
* need to create a bot via [botfather|https://telegram.me/BotFather]
* get token from botfather

### Environment
* TELEGRAM_BOT_TOKEN

### Run
```shell
TELEGRAM_BOT_TOKEN={TELEGRAM_BOT_TOKEN} go run .
```

### Containerize
```shell
podman build .
```

### Bot command
* /status report bot status
* /list list keyword
* /add {keyword} add keyword
* /remove {keyword} remove keyword
* /q {keyword} query products in the database