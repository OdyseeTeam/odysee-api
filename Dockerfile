FROM alpine
EXPOSE 8080
EXPOSE 2112

RUN apk update && \
    apk add --no-cache \
    openssh-keygen

WORKDIR /app
COPY dist/lbrytv_linux_amd64/lbrytv /app
COPY ./docker/lbrytv.yml ./config/lbrytv.yml
COPY ./docker/launcher.sh ./

CMD ["./launcher.sh"]
