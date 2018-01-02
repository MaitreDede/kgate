package server

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/hashicorp/yamux"
	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"

	"github.com/mcluseau/kgate/common"
)

var (
	httpBindSpec = "127.0.0.1:1081"

	certFile,
	keyFile,
	caCertFile string

	crts    []tls.Certificate
	rootCAs = x509.NewCertPool()

	remote *yamux.Session
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use: "server",
		Run: run,
	}

	flags := cmd.Flags()
	flags.StringVar(&httpBindSpec, "http", httpBindSpec, "HTTP listen spec")
	flags.StringVar(&certFile, "crt", "server.crt", "Certificate file")
	flags.StringVar(&keyFile, "key", "server.key", "Key file")
	flags.StringVar(&caCertFile, "ca", "ca.crt", "CA certificate file")
	common.RegisterFlags(flags)

	return cmd
}

func run(cmd *cobra.Command, args []string) {
	crt, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal("Failed to load X509 key/crt: ", err)
	}
	crts = []tls.Certificate{crt}

	caBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatal("Failed to read CA certificate: ", err)
	}
	if !rootCAs.AppendCertsFromPEM(caBytes) {
		log.Fatal("Failed to parse CA certificate.")
	}

	log.Print("Listening on ", httpBindSpec)
	log.Fatal(http.ListenAndServe(httpBindSpec, websocket.Handler(handleWS)))
}

func handleWS(ws *websocket.Conn) {
	handleConnection(ws)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	safeConn := tls.Server(conn, &tls.Config{
		Certificates: crts,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs,
	})

	session, err := yamux.Server(safeConn, nil)
	if err != nil {
		log.Print("yamu.Server() failed: ", err)
		return
	}

	common.ManageSession(session)
}
