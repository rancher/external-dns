FROM alpine:3.4
RUN apk add --no-cache ca-certificates 
COPY external-dns /usr/bin/
ENTRYPOINT ["/usr/bin/external-dns"]
