# syntax=docker/dockerfile:1

FROM alpine:3.16 AS gather

WORKDIR /build

ADD https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz ./
RUN tar -xf ffmpeg-git-amd64-static.tar.xz && mv ffmpeg-*-static/ffprobe ffmpeg-*-static/ffmpeg ./

RUN chmod a+x ffmpeg ffprobe

FROM alpine:3.16 AS build
EXPOSE 8080

RUN apk update && \
    apk add --no-cache \
    openssh-keygen

COPY --from=gather /build/ffprobe /usr/local/bin/

WORKDIR /app
COPY dist/linux_amd64/oapi /app
RUN chmod a+x /app/oapi
COPY ./docker/oapi.yml ./config/oapi.yml
COPY ./docker/launcher.sh ./

CMD ["./launcher.sh"]
