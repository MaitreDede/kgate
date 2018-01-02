package client

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"

	"github.com/mcluseau/kgate/common"
)

var (
	dialTimeout = 10 * time.Second

	bindSpec       = "127.0.0.1:1080"
	gateway        = "ws://localhost:1081"
	proxyUrl       = ""
	safeServerName = "localhost"
	tlsKey         = "client.key"
	tlsCert        = "client.crt"
	caCertFile     = "ca.crt"

	rootCAs = x509.NewCertPool()

	remote *yamux.Session
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use: "client",
		Run: run,
	}

	flags := cmd.Flags()
	flags.StringVar(&bindSpec, "bind", bindSpec, "Bind address")
	flags.StringVar(&gateway, "gw", gateway, "WebSocket gateway URL")
	flags.StringVar(&proxyUrl, "proxy", proxyUrl, "Proxy to reach the gateway")
	flags.StringVar(&safeServerName, "safe-server-name", safeServerName, "Server name for the safe tunnel")
	flags.StringVar(&tlsKey, "key", tlsKey, "Key for TLS auth")
	flags.StringVar(&tlsCert, "crt", tlsCert, "Certificate for TLS auth")
	flags.StringVar(&caCertFile, "ca", caCertFile, "CA certificate for TLS auth")
	common.RegisterFlags(flags)

	return cmd
}

func run(cmd *cobra.Command, args []string) {
	crt, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		log.Fatal("Failed to TLS auth files: ", err)
	}

	caBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatal("Failed to read CA certificate: ", err)
	}
	if !rootCAs.AppendCertsFromPEM(caBytes) {
		log.Fatal("Failed to parse CA certificate.")
	}

	var dialer proxy.Dialer = proxy.Direct
	if proxyUrl != "" {
		log.Print("Using proxy")
		var err error
		dialer, err = proxy.SOCKS5("tcp", proxyUrl, nil, proxy.Direct)
		if err != nil {
			log.Fatal("Unable to build the proxy: ", err)
		}
	}

	targetUrl, err := url.Parse(gateway)
	if err != nil {
		log.Fatal("invalid URL: ", gateway, ": ", err)
	}
	wsConfig, err := websocket.NewConfig(gateway, gateway)
	if err != nil {
		log.Fatal("failed to create WS config: ", err)
	}

	log.Print("Connection, stage 0...")
	conn, err := dialer.Dial("tcp", targetUrl.Host)
	if err != nil {
		log.Fatal("Connection stage 0 failed: ", err)
	}
	if targetUrl.Scheme == "wss" {
		conn = tls.Client(conn, &tls.Config{
			ServerName:         strings.Split(targetUrl.Host, ":")[0],
			InsecureSkipVerify: true,
		})
	}

	log.Print("Connection, stage 1...")
	ws, err := websocket.NewClient(wsConfig, conn)
	if err != nil {
		log.Fatal("Connection stage 1 failed: ", err)
	}

	log.Print("Connection, stage 2...")
	safeConn := tls.Client(ws, &tls.Config{
		ServerName:   safeServerName,
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{crt},
	})

	log.Print("Connection, stage 3...")
	session, err := yamux.Client(safeConn, nil)
	if err != nil {
		log.Fatal("Connection stage 3 failed: ", err)
	}

	common.ManageSession(session)
}
