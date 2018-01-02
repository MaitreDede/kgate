package common

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/spf13/pflag"

	"github.com/mcluseau/kgate/config"
)

var (
	dialTimeout = 10 * time.Second

	remote *yamux.Session

	listenersMutex = sync.Mutex{}
	sessionMutex   = sync.Mutex{}

	listenerSpecs []string
	listeners     []*Listener
)

type Listener struct {
	Listen string `json:"listen"`
	Target string `json:"target"`
}

func RegisterFlags(flags *pflag.FlagSet) {
	flags.StringSliceVarP(&listenerSpecs, "local-transfer", "L", nil, "Local port transfers (syntax: <local addr>:<local port>:<remote addr>:<remote port>")
}

func parseListeners() {
	listeners = make([]*Listener, 0)

	cfgEnv := os.Getenv("CONFIG")

	if cfgEnv != "" {
		cfg := &config.Config{}
		if err := json.Unmarshal([]byte(cfgEnv), cfg); err != nil {
			log.Fatal("failed to parse CONFIG env: ", err)
		}

		for port, tr := range cfg.LocalTransfers {
			listeners = append(listeners, &Listener{
				Listen: fmt.Sprintf(":%d", port),
				Target: tr.Target,
			})
		}
	}

	for _, spec := range listenerSpecs {
		parts := strings.Split(spec, ":")

		if len(parts) != 4 {
			log.Fatal("invalid local port transfer spec: ", spec)
		}

		listeners = append(listeners, &Listener{
			Listen: parts[0] + ":" + parts[1],
			Target: parts[2] + ":" + parts[3],
		})
	}
}

func ManageSession(session *yamux.Session) error {
	sessionMutex.Lock()

	if remote != nil {
		remote.Close()
	}

	remote = session

	sessionMutex.Unlock()

	if pingRTT, err := session.Ping(); err == nil {
		log.Print("Session opened (ping: ", pingRTT, ")")
	} else {
		log.Print("Session ping failed: ", err)
		return err
	}

	sessionMutex.Lock()
	listenRemote(session)
	if remote == session {
		remote = nil
	}
	sessionMutex.Unlock()

	return nil
}

func StartListeners() {
	parseListeners()

	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	for _, listener := range listeners {
		l := listener
		go startListener(l.Listen, l.Target)
	}
}

func startListener(bindSpec, target string) {
	log.Print("Listening on ", bindSpec)
	l, err := net.Listen("tcp", bindSpec)
	if err != nil {
		log.Fatal("Fail to listen: ", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept() failed: ", err)
		}

		handleConn(conn, target)
	}
}

func handleConn(conn net.Conn, target string) {
	defer conn.Close()

	session := remote
	if session == nil {
		return
	}

	log.Print("tunneling to ", target)
	defer log.Print("tunneling to ", target, " finished")

	stream, err := session.Open()
	if err != nil {
		log.Print("session open failed: ", err)
		return
	}

	defer stream.Close()

	stream.Write([]byte(target + "\n"))

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		io.Copy(conn, stream)
		closeWrite(conn)
		wg.Done()
	}()

	go func() {
		io.Copy(stream, conn)
		stream.Close()
		wg.Done()
	}()

	wg.Wait()
}

func listenRemote(session *yamux.Session) {
	for {
		conn, err := session.Accept()
		if err != nil {
			log.Print("session.Accept() failed: ", err)
			remote = nil
			return
		}

		go handleClientConnection(conn)
	}
}

func handleClientConnection(conn net.Conn) {
	defer conn.Close()

	bufIn := bufio.NewReader(conn)

	// read the target
	targetAddr, err := bufIn.ReadString('\n')
	if err != nil {
		log.Print("client read error: ", err)
		return
	}

	// TODO validate targetAddr allowance

	// proxy
	targetAddr = targetAddr[:len(targetAddr)-1]

	proxy(conn, bufIn, "tcp", targetAddr)
}

func proxy(conn net.Conn, in io.Reader, proto, targetAddr string) {
	log.Print("proxying to ", targetAddr)
	defer log.Print("proxying to ", targetAddr, " finished")

	target, err := net.DialTimeout(proto, targetAddr, dialTimeout)
	if err != nil {
		log.Printf("dial %s to %s failed: %v", proto, targetAddr, err)
		return
	}

	defer target.Close()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		io.Copy(conn, target)
		closeWrite(conn)
		wg.Done()
	}()

	go func() {
		io.Copy(target, in)
		target.Close()
		wg.Done()
	}()

	wg.Wait()
}

type closeWriter interface {
	CloseWrite() error
}

func closeWrite(x interface{}) {
	if cw, ok := x.(closeWriter); ok {
		cw.CloseWrite()
	}
}
