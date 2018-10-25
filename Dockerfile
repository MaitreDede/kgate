from golang:1.11.1-alpine3.8 as build-env
run apk add --update gcc musl-dev
env pkg github.com/mcluseau/kgate
add . ${GOPATH}/src/${pkg}
run cd ${GOPATH}/src/${pkg} \
 && go vet -composites=false ./... \
 && go test ./... \
 && go install

from alpine:3.8
run apk add --update ca-certificates
entrypoint ["/kgate"]
copy --from=build-env /go/bin/kgate /
