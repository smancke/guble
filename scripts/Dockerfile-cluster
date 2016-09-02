# this Dockerfile requires u to build the app locally with the name `guble`
FROM phusion/baseimage
RUN mkdir -p /var/lib/guble

COPY guble /go/bin/app


EXPOSE 10000 8080
VOLUME /var/lib/guble


ENTRYPOINT ['/go/bin/app']
