FROM alpine
COPY ./guble /usr/local/bin/guble
COPY ./guble-cli/guble-cli /usr/local/bin/guble-cli
RUN mkdir -p /var/lib/guble
VOLUME ["/var/lib/guble"]
ENTRYPOINT ["/usr/local/bin/guble"]
EXPOSE 8080
