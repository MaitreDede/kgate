# kgate

Gateway to and from Kubernetes resources.

Kgate allows you to expose local network resources to a remote Kubernetes cluster, or resource from this cluster a internal resources. Works over TCP or the websocket protocol for maximum interoperability.

The intend is around Kubernetes but it doesn't not really depend on it, just on common mecanisms present in it.
