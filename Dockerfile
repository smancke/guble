FROM golang:onbuild

ENTRYPOINT ["/go/bin/app", "--kv-backend", "memory"]

EXPOSE 8080