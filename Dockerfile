FROM alpine
EXPOSE 80

RUN mkdir /app /static
WORKDIR /app
COPY dist/linux_amd64/lbryweb /app
COPY lbryweb.docker.yml /app/lbryweb.yml

CMD ["/app/lbryweb", "serve"]
