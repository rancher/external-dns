go-infoblox
=========
This project implements a Go client library for the Infoblox WAPI.  This
library supports version 1.4.1 and user/pass auth.

Work in Progress! Not for use in production, but do let me know if you find
that it suits your needs.

Installing
----------
Run

    go get github.com/fanatic/go-infoblox

Include in your source:

    import "github.com/fanatic/go-infoblox"

Godoc
-----
See http://godoc.org/github.com/fanatic/go-infoblox

Using
-----

    go run ./example/example.go

Debugging
---------
To see what requests are being issued by the library, set up an HTTP proxy
such as Charles Proxy and then set the following environment variable:

    export HTTP_PROXY=http://localhost:8888

To Do
-----
- Only supports Network, Record:Host, Record:Cname, and Record:Ptr - need to add other WAPI objects, but they should be trivial to add.
- Unit tests
- Responses as objects rather than interfaces
