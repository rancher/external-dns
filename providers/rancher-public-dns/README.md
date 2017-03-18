# rancher-public-dns provider

## Configuration
The provider requires the following environment variables to be set:

* `NAME_TEMPLATE`: `%{{service_name}}.%{{stack_name}}`
* `TTL`: `60`
* `RANCHER_PUBLIC_DNS_URL`: `http://<PUBLIC_DNS_SERVICE_HOSTNAME>:8095/v1-rancher-dns/`

## Running

To use this provider build and run with:

> $ ./external-dns -provider=rancher-public-dns
