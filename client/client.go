package main

import (
	"io"
	"log"
	"net"
	"os"
)

func mustCopy(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatalf("io.Copy error: %v\n", err)
	}
}

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("net.Dial error: %v\n", err)
	}

	done := make(chan bool)
	defer close(done)

	// Goroutine for prints out the messages from the server, this goroutine will drain the server's messages before quitting
	go func() {
		mustCopy(os.Stdout, conn)
		log.Println("Done listening, quitting...")
		select {
		case done <- true:
		default:
			conn.Close()
			close(done)
			os.Exit(1)
		}
	}()

	// mustCopy will block
	mustCopy(conn, os.Stdin)
	conn.Close()
	// Only ends the app when the above goroutine is done
	<-done
	close(done)
}
