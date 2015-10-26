FROM ubuntu:15.04
MAINTAINER Rancher Labs, Inc.
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
    ca-certificates

#RUN curl -sSL -o external-dns.tar.gz https://github.com/rancher/externa-dns/releases/download/v0.1/external-dns.tar.gz; tar -xvzf external-dns.tar.gz; mv external-dns/external-dns /usr/bin/external-dns
COPY ./bin/external-dns /usr/bin/

COPY launch /
ENTRYPOINT ["/launch"]