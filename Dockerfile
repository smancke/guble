FROM busybox:glibc
ADD guble guble
ENTRYPOINT ["/guble"]
RUN mkdir -p /var/lib/guble
VOLUME ["/var/lib/guble"]
EXPOSE 8080
