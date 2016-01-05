FROM golang:onbuild

RUN go install github.com/smancke/guble/guble-cli
ENTRYPOINT ["/go/bin/app"]

VOLUME ["/var/lib/guble"]

EXPOSE 8080