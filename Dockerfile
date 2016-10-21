FROM golang:1.6
MAINTAINER Jack Twilley
RUN go get github.com/mathuin/external-dns
CMD ["/go/bin/external-dns"]
