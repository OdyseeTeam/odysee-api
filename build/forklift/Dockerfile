# syntax=docker/dockerfile:1
FROM odyseeteam/transcoder-ffmpeg:5.1.1 AS ffmpeg
FROM alpine:3.17
EXPOSE 8080

COPY --from=ffmpeg /build/ffprobe /usr/local/bin/

WORKDIR /app
COPY ./dist/linux_amd64/forklift /app
COPY ./apps/forklift/config/forklift.yml /app/config/forklift.yml

CMD ["/app/forklift", "serve"]
