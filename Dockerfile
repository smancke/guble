FROM scratch
ADD guble guble
ENTRYPOINT ["/guble"]
VOLUME ["/var/lib/guble"]
EXPOSE 8080
