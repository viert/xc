#!/usr/bin/env python3
import os
import sys
from bot.bot import Bot

ICQ_TOKEN = os.environ.get("ICQ_TOKEN")
ICQ_CHAT_ID = os.environ.get("ICQ_CHAT_ID")
ICQ_API_URL = os.environ.get("ICQ_API_URL", "https://api.icq.net/bot/v1")

def send(message):
    if not ICQ_TOKEN:
        raise RuntimeError("No ICQ_TOKEN variable defined")
    if not ICQ_CHAT_ID:
        raise RuntimeError("No ICQ_CHAT_ID variable defined")
    if not message:
        raise ValueError("message is empty")

    print("using api base %s" % ICQ_API_URL)
    bot = Bot(token=ICQ_TOKEN, api_url_base=ICQ_API_URL)
    r = bot.send_text(chat_id=ICQ_CHAT_ID, text=message)
    print("response was:", r)


if __name__ == '__main__':
    message = ' '.join(sys.argv[1:]).strip().replace("\\n", "\n")
    send(message)
