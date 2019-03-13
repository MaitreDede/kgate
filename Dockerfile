# ------------------------------------------------------------------------
from golang:1.12.0-alpine3.9 as build-env

arg GOPROXY
env CGO_ENABLED 0

workdir /src
add go.mod go.sum ./
run go mod download

add . ./
run go test ./...
run go install .

# ------------------------------------------------------------------------
from alpine:3.9
entrypoint ["/bin/kgate"]
run apk add --update ca-certificates
copy --from=build-env /go/bin/ /bin/
