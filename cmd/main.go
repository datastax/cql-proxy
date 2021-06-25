package main

import (
	"context"
	"cql-proxy/proxy"
	"cql-proxy/proxycore"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

func cancelExample() {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Add(4)

	for i := 0; i < 4; i++ {
		go func() {
			done := false
			for !done {
				fmt.Println("looping...")
				select {
				case <-ctx.Done():
					err := ctx.Err()
					if err == context.Canceled {
						log.Println("cancelled")
					} else {
						log.Printf("error: %s\n", err)
					}
					done = true
				}
			}
			wg.Done()
		}()
	}

	time.Sleep(5 * time.Second)

	cancel()

	wg.Wait()
}

func closeExample() {
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:8000")
		if err != nil {
			log.Fatalf("unable to listen %v", err)
		}

		wg.Done()

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("unable to accept new connection %v", err)
			}

			go func(c net.Conn) {
				timer := time.NewTimer(2 * time.Second)
				done := false
				for !done {
					select {
					case <-timer.C:
						done = true
					default:
						c.Write([]byte("a"))
					}
				}
				log.Println("closing connection")
				c.Close()
			}(conn)
		}

	}()

	wg.Wait()

	conn, err := net.Dial("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatalf("unable to connect %v", err)
	}

	for {
		b := make([]byte, 16)
		_, err := conn.Read(b)
		if err == net.ErrClosed || err == io.EOF {
			log.Println("closed")
			break
		} else if err != nil {
			log.Printf("error reading %v\n", err)
		}
		//log.Println(string(b[:n]))
	}
}

func singleChannelExample() {

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	type Event struct {
		t string
	}

	ch := make(chan Event)

	wg.Add(4)

	for i := 0; i < 4; i++ {
		go func(i int) {
			done := false
			for !done {
				fmt.Println("looping...")
				select {
				case evt := <-ch:
					fmt.Printf("In goroutine %d: %v\n", i, evt)
				case <-ctx.Done():
					err := ctx.Err()
					if err == context.Canceled {
						log.Println("cancelled")
					} else {
						log.Printf("error: %s\n", err)
					}
					done = true
				}
			}
			wg.Done()
		}(i + 1)
	}

	for i := 0; i < 100; i++ {
		ch <- Event{t: "schema"}
	}

	time.Sleep(1 * time.Second)

	cancel()

	wg.Wait()
}

type ReceiverFunc func(io.Reader) error

func (f ReceiverFunc) Receive(r io.Reader) error {
	return f(r)
}

type SenderFunc func(io.Writer) error

func (f SenderFunc) send(w io.Writer) error {
	return f(w)
}

func (f SenderFunc) Closing(err error) {
	log.Printf("################################### closing %v ##################################", err)
}

func connWithBundleEx() {
	bundle, err := proxycore.LoadBundleZip("secure-connect-testdb1.zip")
	if err != nil {
		log.Fatalf("unable to open bundle: %v", err)
	}

	factory, err := proxycore.ResolveAstra(bundle)
	if err != nil {
		log.Fatalf("unable to resolve endpoints: %v", err)
	}

	ctx := context.Background()

	conn, err := proxycore.ConnectClient(ctx, factory.ContactPoints()[0])
	if err != nil {
		log.Fatalf("unable to connect to cluster: %v", err)
	}

	auth := proxycore.NewDefaultAuth("HYhtHNEYMKOFpFGyOsAYyHSK", "rEPtSneDWH3Of8HCMQD1d8uANl5.T5NavwIvJLLUivOJsA7fyl9z_4uTNCmHMkgiWcPTz2nCI5,p+3X41hEpdj5fDz,tOa,vjEMmd0K,2wllbPn_dqRZPox5TbP1H,QE")
	version, err := conn.Handshake(ctx, primitive.ProtocolVersion4, auth)
	if err != nil {
		log.Fatalf("unable to connect to cluster: %v", err)
	}
	_ = version

	timer := time.NewTimer(time.Second)

	closed := false
	for !closed {
		select {
		case <-conn.IsClosed():
			log.Printf("closed %v", conn.Err())
			closed = true
		case <-timer.C:
			conn.Close()
		}
	}

	time.Sleep(2 * time.Second)
}

func connClusterWithBundleEx() {
	//bundle, err := proxycore.LoadBundleZip("secure-connect-testdb1.zip")
	//if err != nil {
	//	log.Fatalf("unable to open bundle: %v", err)
	//}

	//factory, err := proxycore.ResolveAstra(bundle)
	//if err != nil {
	//	log.Fatalf("unable to resolve astra: %v", err)
	//}

	factory, err := proxycore.Resolve("127.0.0.1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	auth := proxycore.NewDefaultAuth("HYhtHNEYMKOFpFGyOsAYyHSK", "rEPtSneDWH3Of8HCMQD1d8uANl5.T5NavwIvJLLUivOJsA7fyl9z_4uTNCmHMkgiWcPTz2nCI5,p+3X41hEpdj5fDz,tOa,vjEMmd0K,2wllbPn_dqRZPox5TbP1H,QE")
	cluster, err := proxycore.ConnectCluster(ctx, proxycore.ClusterConfig{
		Version:         primitive.ProtocolVersion4,
		Auth:            auth,
		Factory:         factory,
		ReconnectPolicy: proxycore.NewReconnectPolicy(),
	})
	if err != nil {
		log.Fatalf("unable to connect to cluster: %v", err)
	}

	session, err := proxycore.ConnectSession(ctx, cluster, proxycore.SessionConfig{
		ReconnectPolicy: proxycore.NewReconnectPolicy(),
		NumConns:        1,
		Version:         primitive.ProtocolVersion4,
		Auth:            auth,
	})

	if err != nil {
		log.Fatalf("unable to connect to cluster: %v", err)
	}

	timer := time.NewTimer(time.Second)

	select {
	case <-session.IsConnected():
		fmt.Println("session is connected")
	}

	closed := false
	for !closed {
		select {
		case <-ctx.Done():
			closed = true
		case <-timer.C:
			cancel()
		}
	}

	time.Sleep(2 * time.Second)
}

func contextTimeoutEx() {
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	err := ctx.Err()
	//time.Sleep(2 * time.Second)
	fmt.Println(err)
}

func main() {
	ctx := context.Background()

	factory, _ := proxycore.Resolve("127.0.0.1")

	p := proxy.NewProxy(ctx, proxy.Config{
		Version:         primitive.ProtocolVersion4,
		Factory:         factory,
		ReconnectPolicy: proxycore.NewReconnectPolicy(),
		NumConns:        1,
	})

	p.ListenAndServe("127.0.0.1:8000")

	//connClusterWithBundleEx()
}
