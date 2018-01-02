package common

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/spf13/pflag"
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

func ParseListeners() {
	listeners = make([]*Listener, 0, len(listenerSpecs))
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
	ParseListeners()

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

	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	wg := sync.WaitGroup{}
	for _, listener := range listeners {
		l := listener
		wg.Add(1)
		go func() {
			startListener(l.Listen, l.Target)
			wg.Done()
		}()
	}

	wg.Add(1)
	go listenRemote(session)

	wg.Wait()

	log.Print("Session finished.")
	return nil
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
	log.Print("connecting to ", target)
	defer log.Print("connection to ", target, " finished")

	defer conn.Close()

	session := remote
	if session == nil {
		return
	}

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
