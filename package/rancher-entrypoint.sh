#!/bin/bash

# first arg is `-flag` or `--some-flag`
if [ "${1:0:1}" = '-' ]; then
	set -- /usr/bin/external-dns "$@"
fi

# no argument
if [ -z "$1" ]; then
	set -- /usr/bin/external-dns
fi

/usr/bin/update-rancher-ssl

exec "$@"
