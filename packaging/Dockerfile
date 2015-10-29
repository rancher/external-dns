FROM ubuntu:15.04
MAINTAINER Rancher Labs, Inc.
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
    ca-certificates curl

RUN curl -sSL -o external-dns.tar.gz https://github.com/rancher/external-dns/releases/download/v0.1.0/external-dns.tar.gz && tar -xvzf external-dns.tar.gz && dir=`tar tvf external-dns.tar.gz | awk 'NR==1{print $6}'` && mv $dir/bin/external-dns /usr/bin/external-dns

#COPY ./bin/external-dns /usr/bin/

ENTRYPOINT ["/usr/bin/external-dns"]
