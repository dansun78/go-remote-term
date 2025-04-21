package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/dansun78/go-remote-term/internal/terminal"
)

var (
	addr     = flag.String("addr", ":8080", "HTTP service address")
	certFile = flag.String("cert", "", "TLS cert file path")
	keyFile  = flag.String("key", "", "TLS key file path")
)

func main() {
	flag.Parse()

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Terminal WebSocket handler
	http.HandleFunc("/ws", terminal.HandleWebSocket)

	// Start the server
	fmt.Printf("Starting remote terminal server on %s\n", *addr)
	var err error
	if *certFile != "" && *keyFile != "" {
		fmt.Println("Using HTTPS")
		err = http.ListenAndServeTLS(*addr, *certFile, *keyFile, nil)
	} else {
		fmt.Println("WARNING: Using HTTP (insecure)")
		err = http.ListenAndServe(*addr, nil)
	}

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
