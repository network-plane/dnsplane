package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"

	// "github.com/bettercap/readline"
	"github.com/chzyer/readline"
)

func setupUnixSocketListener(socketPath string) {
	// Ensure there's no existing UNIX socket with the same name
	err := syscall.Unlink(socketPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal("Error removing existing UNIX socket:", err)
	}

	// Create the UNIX domain socket and listen for incoming connections
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("Error setting up UNIX socket listener:", err)
	}
	defer listener.Close()

	log.Printf("Listening on UNIX socket at %s", socketPath)

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Handle each connection in a separate goroutine
		go func(c net.Conn) {
			defer c.Close() // Ensure the connection is closed after processing

			buf := make([]byte, 1024) // Buffer for reading data from the connection
			n, err := c.Read(buf)     // Read the incoming data
			if err != nil {
				log.Printf("Error reading from connection: %v", err)
				return
			}

			command := string(buf[:n]) // Convert buffer to a string for command processing
			log.Printf("Received command: %s", command)
		}(conn) // Start the goroutine with the current connection
	}
}

func connectToUnixSocket(socketPath string) {
	conn, err := net.Dial("unix", socketPath) // Connect to the UNIX socket
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to UNIX socket: %v\n", err)
		return
	}
	defer conn.Close() // Ensure connection closure

	fmt.Println("Connected to UNIX socket:", socketPath)

	// Interactive mode setup
	rl, err := readline.NewEx(&rlconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return
	}
	defer rl.Close() // Close readline when done

	handleCommandLoop(rl) // Call the function for command handling
}
