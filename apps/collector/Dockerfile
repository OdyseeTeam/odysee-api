FROM alpine
EXPOSE 8080

WORKDIR /app
COPY ./dist/collector_linux_amd64/collector /app
# COPY ./collector.yml ./collector.yml

CMD ["./collector"]
