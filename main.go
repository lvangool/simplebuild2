package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
)

type SysInfo struct {
	Hostname  string
	Node      string
	IP        []string
	CallerIP  string
	UpSince   time.Time
	Version   string
	Requests  int
	Namespace string
}

var (
	port     int
	destPort int
	destIP   string
)

const VERSION = "0.5"

func main() {
	flag.IntVar(&port, "port", 8050, "serve on port")
	flag.IntVar(&destPort, "dport", 8050, "destination port")
	flag.StringVar(&destIP, "dip", "localhost", "destination IP")
	flag.Parse()

	log.Printf("Listening on %d\n", port)

	sysInfo := &SysInfo{
		UpSince:  time.Now().UTC(),
		Version:  VERSION,
		Requests: 0,
	}
	sysInfo.Hostname, _ = os.Hostname()
	sysInfo.Node = os.Getenv("SERVER_NAME")
	sysInfo.Namespace = os.Getenv("NAMESPACE")

	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				sysInfo.IP = append(sysInfo.IP, v.IP.String())
			case *net.IPAddr:
				sysInfo.IP = append(sysInfo.IP, v.IP.String())
			}
		}
	}
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	api.SetApp(rest.AppSimple(func(w rest.ResponseWriter, r *rest.Request) {
		sysInfo.CallerIP = r.RemoteAddr
		sysInfo.Requests = sysInfo.Requests + 1
		w.WriteJson(sysInfo)
	}))

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	// start the pinger
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// do stuff
				log.Println("Checking siblings...")
				response, err := client.Get(fmt.Sprintf("http://%s:%d/", destIP, destPort))
				if err != nil {
					log.Printf("Error during HTTP Get Ping %s\n", err.Error())
				} else if response.StatusCode != 200 {
					log.Printf("HTTP Get returned %d\n", response.StatusCode)
				} else {
					log.Println("200 OK")
				}
			case <-c:
				log.Println("Received termination signal")
				ticker.Stop()
				os.Exit(1)
				return
			}
		}
	}()

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), api.MakeHandler()))
}
