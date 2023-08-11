# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


FROM golang:1.19.4-alpine AS builder

WORKDIR /usr/src/app/

# needed with contrast-go ?!
RUN apk add make gcc musl-dev gcompat

RUN apk add build-base protobuf-dev protoc

# install contrast-go
RUN go run github.com/contrast-security-oss/contrast-go-installer@latest latest

COPY ./src/productcatalogservice/ ./
RUN contrast-go build -o /go/bin/productcatalogservice/

# -----------------------------------------------------------------------------

FROM alpine AS release

WORKDIR /usr/src/app/

COPY ./src/productcatalogservice/contrast_security.yaml /usr/src/app
COPY ./src/productcatalogservice/products.json ./
COPY --from=builder /go/bin/productcatalogservice/ ./

EXPOSE ${PRODUCT_SERVICE_PORT}
ENTRYPOINT [ "./productcatalogservice" ]
