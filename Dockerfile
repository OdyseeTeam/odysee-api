FROM alpine
EXPOSE 8080
EXPOSE 2112

RUN apk update && \
    apk add --no-cache \
    openssh-keygen

WORKDIR /app
COPY dist/linux_amd64/lbrytv /app
RUN chmod a+x /app/lbrytv
COPY ./docker/lbrytv.yml ./config/lbrytv.yml
COPY ./docker/launcher.sh ./

CMD ["./launcher.sh"]
