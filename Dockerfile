FROM golang:onbuild

ENTRYPOINT ["/go/bin/app"]

EXPOSE 8080