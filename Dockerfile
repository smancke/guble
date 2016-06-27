FROM busybox:glibc
ADD guble guble
RUN mkdir -p /var/lib/guble
RUN mkdir -p /lib/x86_64-linux-gnu
COPY /lib/x86_64-linux-gnu/libdl-2.23.so /lib/x86_64-linux-gnu/libdl-2.23.so
RUN ln -s /lib/x86_64-linux-gnu/libdl-2.23.so /lib/x86_64-linux-gnu/libdl.so.2
VOLUME ["/var/lib/guble"]
ENTRYPOINT ["/guble"]
EXPOSE 8080
