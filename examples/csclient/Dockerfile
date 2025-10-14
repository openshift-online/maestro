FROM golang:1.24 AS builder

ENV SOURCE_DIR=/csclient
WORKDIR $SOURCE_DIR
COPY . $SOURCE_DIR

RUN go build -o csclient examples/csclient/main.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

RUN microdnf update -y && \
    microdnf install -y util-linux && \
    microdnf clean all

COPY --from=builder /csclient/csclient /usr/local/bin/

EXPOSE 8000

ENTRYPOINT ["/usr/local/bin/csclient", "server"]

LABEL name="csclient" \
      vendor="Red Hat, Inc." \
      version="0.0.1" \
      summary="csclient" \
      description="csclient" \
      io.k8s.description="csclient" \
      io.k8s.display-name="csclient" \
      io.openshift.tags="csclient"
