FROM golang:onbuild

RUN go install github.com/smancke/guble/guble-cli
ENTRYPOINT ["/go/bin/app"]

EXPOSE 8080