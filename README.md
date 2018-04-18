# kgate

ALPHA WITH KNOWN ISSUES ;-)

Gateway to and from Kubernetes resources.

Kgate allows you to expose local network resources to a remote Kubernetes cluster, or resource from this cluster as internal resources. Works over TCP or the websocket protocol for maximum interoperability.

The intend is around Kubernetes but it doesn't not really depend on it, just on common mecanisms present in it.

## Usage

```
# run a server in a namespace
kgatectl -n my-ns init

# expose remote ports (will restart the server)
kgatectl -n my-ns expose-remote --service as400 --local-port 23 --remote-target 127.0.0.1:23

# create the config file for the client
kgatectl -n my-ns gen-key
```
