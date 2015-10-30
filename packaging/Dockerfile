FROM alpine:3.2
MAINTAINER Rancher Labs, Inc.
RUN apk add --update ca-certificates 

ENV EXT_DNS_RELEASE v0.1.0
ADD https://github.com/rancher/external-dns/releases/download/${EXT_DNS_RELEASE}/external-dns.tar.gz /external-dns.tar.gz
RUN tar -zxvf /external-dns.tar.gz -C /usr/bin

ENTRYPOINT ["/usr/bin/external-dns"]
