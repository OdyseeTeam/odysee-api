FROM alpine
EXPOSE 8080

WORKDIR /app
COPY ./dist/linux_amd64/watchman /app

CMD ["/app/watchman", "serve"]
