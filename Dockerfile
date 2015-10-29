FROM golang:1.5
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
