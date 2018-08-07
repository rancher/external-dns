FROM alpine:3.5
LABEL maintainer "Rancher Labs, Inc."

RUN apk upgrade --no-cache && \
    apk add --no-cache ca-certificates openssl bash wget

ENV SSL_SCRIPT_COMMIT 08278ace626ada71384fc949bd637f4c15b03b53
RUN wget -O /usr/bin/update-rancher-ssl https://raw.githubusercontent.com/rancher/rancher/${SSL_SCRIPT_COMMIT}/server/bin/update-rancher-ssl && \
    chmod +x /usr/bin/update-rancher-ssl

COPY rancher-entrypoint.sh /usr/bin/
COPY external-dns /usr/bin/

ENTRYPOINT ["/usr/bin/rancher-entrypoint.sh"]
