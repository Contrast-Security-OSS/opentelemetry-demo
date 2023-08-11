# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


FROM golang:1.19.2-alpine AS builder
WORKDIR /usr/src/app/

# needed with contrast-go ?!
RUN apk add make gcc musl-dev gcompat

# install contrast-go
RUN go run github.com/contrast-security-oss/contrast-go-installer@latest latest

COPY ./src/checkoutservice/ ./
RUN contrast-go build -o /go/bin/checkoutservice/

# -----------------------------------------------------------------------------

FROM alpine

WORKDIR /usr/src/app/

COPY ./src/checkoutservice/contrast_security.yaml /usr/src/app
COPY --from=builder /go/bin/checkoutservice/ ./

EXPOSE ${CHECKOUT_SERVICE_PORT}
ENTRYPOINT [ "./checkoutservice" ]
