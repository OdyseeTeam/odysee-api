# syntax=docker/dockerfile:1
FROM alpine:3.17
EXPOSE 8080

# RUN apk add --no-cache libc6-compat

WORKDIR /app
COPY ./dist/linux_amd64/uploads /app
COPY ./apps/uploads/config/uploads.yml /app

CMD ["/app/uploads", "serve"]
