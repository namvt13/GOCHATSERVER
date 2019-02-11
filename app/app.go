package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// client is a type string sending-only channel
type client chan<- string
type chObj struct {
	sender  client
	content string
}

var (
	enterCh    = make(chan chObj)
	leaveCh    = make(chan chObj)
	messagesCh = make(chan chObj)
)

func broadcaster() {
	// List of all connected clients
	clientsCh := make(map[client]string)
	for {
		select {
		case chObjCh := <-messagesCh:
			// Receive and broadcast to all other active clients
			for ch := range clientsCh {
				if ch != chObjCh.sender {
					// Drop if the client's channel is not ready to accept new message
					select {
					case ch <- chObjCh.content:
					default:
						continue
					}
				}
			}
		case chObjCh := <-enterCh:
			clientsCh[chObjCh.sender] = chObjCh.content
			clientList := []string{}
			for _, id := range clientsCh {
				clientList = append(clientList, id)
			}
			chObjCh.sender <- fmt.Sprintf("Current clients: [%v]\n", strings.Join(clientList, ", "))
		case chObjCh := <-leaveCh:
			delete(clientsCh, chObjCh.sender)
			close(chObjCh.sender)
		}
	}
}

// Receive all message from the ch channel and writes back to the client
func clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintf(conn, msg)
	}
}

// Create client channel and handle connection
func connHandler(conn net.Conn) {
	// Set timer and another goroutine to handle connection close.
	disconnect := make(chan int)
	go func() {
		timer := time.NewTicker(5 * time.Minute)
		for {
			select {
			case signal := <-disconnect:
				timer.Stop()
				if signal == 2 {
					conn.Close()
				} else if signal == 1 {
					timer = time.NewTicker(5 * time.Minute)
				}
			case <-timer.C:
				conn.Close()
				timer.Stop()
			}
		}
	}()

	client := make(chan string)
	// clientWriter receive message from client channel and write them back to the client
	go clientWriter(conn, client)

	id := ""
	client <- fmt.Sprintln("Please enter your username")

	// Scan for new input
	input := bufio.NewScanner(conn)
	buffer := []chObj{}
	for input.Scan() {
		disconnect <- 1
		if id != "" {
			msgObj := chObj{
				sender:  client,
				content: fmt.Sprintln(id + ": " + input.Text()),
			}
			// Appends message to buffer in case sending to messagesCh encounters problems
			buffer = append(buffer, msgObj)
			t := time.NewTicker(5 * time.Second)
			for len(buffer) > 0 {
				select {
				case messagesCh <- buffer[0]:
					buffer = buffer[1:]
				case <-t.C:
					break
				}
			}
		} else {
			id = input.Text()
			enterCh <- chObj{
				sender:  client,
				content: id,
			}
			client <- fmt.Sprintf("Your are %s\n", id)
			messagesCh <- chObj{
				sender:  client,
				content: fmt.Sprintln(id + " has joined the chat"),
			}
		}
	}
	if err := input.Err(); err != nil {
		fmt.Printf("%s scanning error: %v\n", id, err)
	}

	// Client leave
	leaveCh <- chObj{
		sender:  client,
		content: id,
	}
	messagesCh <- chObj{
		sender:  nil,
		content: fmt.Sprintln(id + " has left the chat"),
	}

	disconnect <- 2
	close(disconnect)
}

func main() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("net.Listen error: %v\n", err)
	}
	defer func() {
		err := listener.Close()
		fmt.Println(err)
	}()

	go broadcaster()

	fmt.Println("Chat server is listening @ http://localhost:8080")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("%s listening error: %v\n", conn.RemoteAddr().String(), err)
			continue
		}
		go connHandler(conn)
	}
}
