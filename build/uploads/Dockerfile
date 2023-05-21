FROM alpine:3.17
EXPOSE 8080

RUN apk add --no-cache libc6-compat

WORKDIR /app
COPY ../dist/linux_amd64/uploads /app

CMD ["/app/uploads", "serve"]
