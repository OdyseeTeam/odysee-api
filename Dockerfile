FROM alpine
EXPOSE 8080
EXPOSE 2112

WORKDIR /app
COPY dist/lbrytv_linux_amd64/lbrytv /app
COPY ./lbrytv.yml ./lbrytv.yml
COPY ./scripts/launcher.sh ./

CMD ["./launcher.sh"]
