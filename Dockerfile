# ------------------------------------------------------------------------
FROM golang:1.12.8-alpine3.9 as build-env

ARG GOPROXY
ENV CGO_ENABLED 0

WORKDIR /src
ADD go.mod go.sum ./
RUN go mod download

ADD . ./
RUN go test ./...
RUN go install .

# ------------------------------------------------------------------------
FROM alpine:3.10.1
ENTRYPOINT ["/bin/kgate"]
RUN apk add --update --no-cache ca-certificates
COPY --from=build-env /go/bin/ /bin/
