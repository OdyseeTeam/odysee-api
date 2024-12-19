# syntax=docker/dockerfile:1
FROM odyseeteam/transcoder-ffmpeg:5.1.1 AS ffmpeg
FROM alpine:3.21
EXPOSE 8080

RUN apk update && \
    apk add --no-cache \
    openssh-keygen

COPY --from=ffmpeg /build/ffprobe /usr/local/bin/

WORKDIR /app
COPY dist/linux_amd64/oapi /app
RUN chmod a+x /app/oapi
COPY ./docker/oapi.yml ./config/oapi.yml
COPY ./docker/launcher.sh ./

CMD ["./launcher.sh"]
