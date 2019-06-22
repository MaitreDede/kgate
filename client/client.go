package client

import (
	"archive/zip"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

type config struct {
	url            string
	safeServerName string
	caBytes        []byte
	certificate    tls.Certificate
}

func run(cmd *cobra.Command, args []string) {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-c

		os.Exit(0)
	}()

	cfg := &config{}

	if len(args) == 0 {
		loadConfigFromArgs(cfg)

	} else {
		loadConfigFromZip(args[0], cfg)
	}

	common.StartListeners()

	for {
		connect(cfg)

		log.Print("retry in 5s")
		time.Sleep(5 * time.Second)
	}
}

func loadConfigFromArgs(cfg *config) {
	crt, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		log.Fatal("Failed to TLS auth files: ", err)
	}

	caBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatal("Failed to read CA certificate: ", err)
	}

	cfg.url = gateway
	cfg.safeServerName = safeServerName
	cfg.caBytes = caBytes
	cfg.certificate = crt
}

func loadConfigFromZip(file string, cfg *config) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		log.Fatal(err)
	}

	defer zr.Close()

	var crtPEM, keyPEM []byte

	for _, f := range zr.File {
		zf, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}

		data, err := ioutil.ReadAll(zf)
		if err != nil {
			log.Fatal(err)
		}

		switch f.Name {
		case "url":
			cfg.url = string(data)
		case "server-name":
			cfg.safeServerName = string(data)
		case "client.crt":
			crtPEM = data
		case "client.key":
			keyPEM = data
		case "ca.crt":
			cfg.caBytes = data
		}
	}

	if cfg.url == "" {
		log.Fatal("no url in config file")
	}

	if cfg.safeServerName == "" {
		log.Fatal("no server-name in config file")
	}

	if crtPEM == nil {
		log.Fatal("no client.crt in config file")
	}

	if keyPEM == nil {
		log.Fatal("no client.key in config file")
	}

	if cfg.caBytes == nil {
		log.Fatal("no ca.crt in config file")
	}

	crt, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		log.Fatal(err)
	}

	cfg.certificate = crt
}

func connect(cfg *config) {
	crt := cfg.certificate

	if !rootCAs.AppendCertsFromPEM(cfg.caBytes) {
		log.Print("Failed to parse CA certificate.")
		return
	}

	var dialer proxy.Dialer = proxy.Direct
	if proxyUrl != "" {
		log.Print("Using proxy")
		var err error
		var url *url.URL
		url, err = url.Parse(proxyUrl)
		if err != nil {
			log.Print("Can't parse proxy url: ", err)
			return
		}
		dialer, err = proxy.FromURL(url, dialer)
		if err != nil {
			log.Print("Unable to build the proxy: ", err)
			return
		}
	}

	targetUrl, err := url.Parse(cfg.url)
	if err != nil {
		log.Print("invalid URL: ", cfg.url, ": ", err)
	}

	wsConfig, err := websocket.NewConfig(cfg.url, cfg.url)
	if err != nil {
		log.Print("failed to create WS config: ", err)
		return
	}

	log.Print("Connection, stage 0... ", cfg.url)
	conn, err := dialer.Dial("tcp", targetUrl.Host)
	if err != nil {
		log.Print("Connection stage 0 failed: ", err)
		return
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
		log.Print("Connection stage 1 failed: ", err)
		return
	}

	log.Print("Connection, stage 2...")
	safeConn := tls.Client(ws, &tls.Config{
		ServerName:   cfg.safeServerName,
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{crt},
	})

	log.Print("Connection, stage 3...")
	session, err := yamux.Client(safeConn, nil)
	if err != nil {
		log.Print("Connection stage 3 failed: ", err)
		return
	}

	common.ManageSession(session)
}
