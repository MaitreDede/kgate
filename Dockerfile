from golang:1.10.1-alpine3.7 as build-env
env pkg github.com/mcluseau/kgate
add . ${GOPATH}/src/${pkg}
run cd ${GOPATH}/src/${pkg} \
 && go vet -composites=false ./... \
 && go test ./... \
 && go install

from alpine:3.7
run apk add --update ca-certificates
entrypoint ["/kgate"]
copy --from=build-env /go/bin/kgate /
