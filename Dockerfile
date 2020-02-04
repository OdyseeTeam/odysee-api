FROM alpine
EXPOSE 8080
EXPOSE 2112

# Needed by updater to connect to github
RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

WORKDIR /app
COPY dist/lbrytv_linux_amd64/lbrytv /app
COPY ./lbrytv.production.yml ./lbrytv.yml
COPY ./scripts/launcher.sh ./

CMD ["./launcher.sh"]
