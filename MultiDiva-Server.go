package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ovandermeer/MultiDiva-Server/internal/ConfigManager"
)

const (
	SERVER_HOST    = "0.0.0.0"
	SERVER_TYPE    = "tcp"
	SERVER_VERSION = "0.1.0"
)

var channelList []chan string
var clientsConnected int
var newClientNum int
var serverQuitting bool

func main() {
	myConfig := ConfigManager.LoadConfig()
	fmt.Println("Server Running...")

	server, err := net.Listen(SERVER_TYPE, SERVER_HOST+":"+myConfig.Port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
	}

	fmt.Println("Listening on " + SERVER_HOST + ":" + myConfig.Port)

	clientsConnected = 0

	InterruptSignal := make(chan os.Signal, 1)
	signal.Notify(InterruptSignal, os.Interrupt)
	go func() {
		for range InterruptSignal {
			closeServer(server, InterruptSignal, channelList)
		}
	}()

	for {
		fmt.Println("Waiting for client...")

		connection, err := server.Accept()
		if !serverQuitting {
			if err != nil {
				fmt.Println("Error accepting: ", err.Error())
			}

			userChannel := make(chan string)

			channelList = append(channelList, userChannel)

			go processClient(connection, userChannel, clientsConnected, newClientNum)
			clientsConnected = clientsConnected + 1
			newClientNum = newClientNum + 1
			fmt.Println("client connected")
		} else {
			break
		}
	}
}

func processClient(connection net.Conn, userChannel chan string, totalClients int, clientNum int) {
	go receiveMessages(connection, userChannel, totalClients, clientNum)
	go sendMessages(connection, userChannel, totalClients, clientNum)
}

func receiveMessages(connection net.Conn, userChannel chan string, totalClients int, clientNum int) {

	for {
		var stringClientMessage string

		buffer := make([]byte, 1024)
		clientMessage, err := connection.Read(buffer)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
			// TODO bandaid fix until i can properly send a logout from client, windows uses "forcibly closed" while linux uses "EOF"
			if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") || strings.Contains(err.Error(), "EOF") {
				fmt.Println("unexpected closure")
				stringClientMessage = "/clientLogout"
			}
		} else {
			stringClientMessage = string(buffer[:clientMessage])
		}

		fmt.Println("Received: ", stringClientMessage, "from client", clientNum)

		if stringClientMessage == "/clientLogout" {
			userChannel <- "/closePipe"
			time.Sleep(10 * time.Millisecond)
			i := 0
			for {
				if channelList[i] == userChannel {
					channelList[i] = channelList[len(channelList)-1]
					channelList[len(channelList)-1] = nil
					channelList = channelList[:len(channelList)-1]
					break
				}
				i = i + 1
			}
			clientsConnected = clientsConnected - 1
			break
		}
		for _, item := range channelList {
			if item != userChannel {
				item <- stringClientMessage
			}
		}
	}
	connection.Close()
}

func sendMessages(connection net.Conn, userChannel chan string, totalClients int, clientNum int) {

	for {

		fmt.Printf("Client %v waiting for message...\n", clientNum)
		message := <-userChannel
		_, err := connection.Write([]byte(message))
		if err != nil {
			fmt.Println("Error writing:", err.Error())
		}
		fmt.Printf("Sent '%s' to client %v !\n", message, clientNum)
		if message == "/closePipe" {
			break
		}
	}
}

func closeServer(server net.Listener, exitSignal chan os.Signal, channelList []chan string) {
	fmt.Println("\nServer shutting down...")

	serverQuitting = true

	for _, item := range channelList {
		item <- "[server] Server shutting down..."
		time.Sleep(1 * time.Millisecond)
		item <- "/closePipe"
	}

	server.Close()
}
