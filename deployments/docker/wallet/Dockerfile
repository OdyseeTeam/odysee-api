FROM debian:buster-slim

# install python, pip, git and clean up
RUN apt-get update && apt-get -y --no-install-recommends install build-essential git python3.7 python3.7-dev python3-pip && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# RUN pip3 install -U pip (broken on pip 10 https://github.com/pypa/pip/issues/5240)
RUN python3.7 -m pip install --upgrade pip setuptools

# get uvloop
RUN python3.7 -m pip install --user uvloop

# copy lbrynet
COPY . /app

# install torba & lbry
WORKDIR /app/torba
RUN python3.7 -m pip install .
WORKDIR /app/lbry
RUN python3.7 -m pip install .
RUN rm ~/.cache -rf

VOLUME /wallet

ENV TCP_PORT=50001
ENV HOST=0.0.0.0
ENV DAEMON_URL=http://lbry:lbry@localhost:9245/
ENV DB_DIRECTORY=/wallet
ENV BANDWIDTH_LIMIT=1000000000000000000000000000000000000000000
ENV MAX_SESSIONS=1000000000
ENV MAX_SEND=1000000000000000000
ENV EVENT_LOOP_POLICY=uvloop
CMD ["/usr/local/bin/torba-server", "lbry.wallet.server.coin.LBC"]
