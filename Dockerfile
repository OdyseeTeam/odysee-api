FROM alpine
EXPOSE 8080

# Needed by updater to connect to github
RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

RUN mkdir /app /static
WORKDIR /app
COPY dist/linux_amd64/lbrytv /app
COPY lbrytv.docker.yml /app/lbrytv.yml

CMD ["/app/lbrytv", "serve"]
