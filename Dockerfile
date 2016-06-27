FROM frolvlad/alpine-glibc
COPY ./guble /usr/local/bin
RUN mkdir -p /var/lib/guble
VOLUME ["/var/lib/guble"]
ENTRYPOINT ["/usr/local/bin/guble"]
EXPOSE 8080
