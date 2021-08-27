FROM alpine
EXPOSE 8080

RUN apk add --no-cache libc6-compat

WORKDIR /app
COPY ./dist/linux_amd64/watchman /app

CMD ["/app/watchman", "serve"]
