FROM golang:1.15
RUN apt update
RUN apt install -y python3 python3-pip
RUN pip3 install mailru-im-bot
ADD icqnotify.py /bin/icqnotify.py
