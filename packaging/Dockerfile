FROM alpine:3.2
MAINTAINER Rancher Labs, Inc.
RUN apk add --update ca-certificates 

COPY ./bin/external-dns /usr/bin/

ENTRYPOINT ["/usr/bin/external-dns"]
