package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bettercap/readline"
	"github.com/miekg/dns"
)

func main() {
	initializeJSONFiles()

	dns.HandleFunc(".", handleRequest)

	dnsServerSettings = getSettings()
	server := &dns.Server{
		Addr: ":53",
		Net:  "udp",
	}

	go func() {
		log.Printf("Starting DNS server on %s\n", server.Addr)
		dnsStats.ServerStartTime = time.Now()
		err := server.ListenAndServe()
		if err != nil {
			fmt.Println("Error starting server:", err)
			os.Exit(1)
		}
	}()

	var currentContext string
	// Setup configuration for readline
	config := readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/readline_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	}

	rl, err := readline.NewEx(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readline: %v\n", err)
		return
	}
	defer rl.Close()

	// Set up command auto-completion
	setupAutocomplete(rl, currentContext)

	for {
		updatePrompt(rl, currentContext) // Update the prompt based on context
		command, err := rl.Readline()
		if err != nil { // Handle EOF or interrupt
			break
		}
		command = strings.TrimSpace(command)
		args := strings.Fields(command)

		if len(args) == 0 {
			continue
		}

		if currentContext == "" {
			// Handle global commands
			switch args[0] {
			case "stats":
				handleStats()
			case "record":
				if len(args) > 1 {
					handleRecord(args, currentContext)
				} else {
					currentContext = "record"
					setupAutocomplete(rl, currentContext)
				}
			case "cache":
				if len(args) > 1 {
					handleCache(args, currentContext)
				} else {
					currentContext = "cache"
					setupAutocomplete(rl, currentContext)
				}
			case "dns":
				if len(args) > 1 {
					handleDNS(args, currentContext)
				} else {
					currentContext = "dns"
					setupAutocomplete(rl, currentContext)
				}
			case "server":
				if len(args) > 1 {
					handleServer(args, currentContext)
				} else {
					currentContext = "server"
					setupAutocomplete(rl, currentContext)
				}
			case "exit", "quit", "q":
				fmt.Println("Shutting down.")
				os.Exit(1)
				return
			case "help", "h", "?":
				mainHelp()
			default:
				fmt.Println("Unknown command:", args[0])
			}
		} else {
			// Handle subcommands
			switch currentContext {
			case "record":
				if args[0] == "/" {
					// Exit from record context
					currentContext = ""
					setupAutocomplete(rl, currentContext)
				} else {
					handleRecord(args, currentContext) // Process record subcommands
				}
			case "cache":
				if args[0] == "/" {
					currentContext = ""
					setupAutocomplete(rl, currentContext) // Change context back to global
				} else {
					handleCache(args, currentContext)
				}
			case "dns":
				if args[0] == "/" {
					currentContext = ""
					setupAutocomplete(rl, currentContext)
				} else {
					handleDNS(args, currentContext)
				}
			case "server":
				if args[0] == "/" {
					currentContext = ""
					setupAutocomplete(rl, currentContext)
				} else {
					handleServer(args, currentContext)
				}
			default:
				fmt.Println("Unknown server subcommand:", args[1])
			}
		}

		// switch args[0] {
		// case "stats":
		// 	handleStats()
		// case "record":
		// 	handleRecord(args)
		// case "cache":
		// 	handleCache(args)
		// case "dns":
		// 	handleDNS(args)
		// case "server":
		// 	handleServer(args)
		// case "exit", "quit", "q":
		// 	fmt.Println("Shutting down.")
		// 	return
		// case "help", "h", "?":
		// 	mainHelp()
		// default:
		// 	fmt.Println("Unknown command:", args[0])
		// }
	}
}

func processAuthoritativeAnswer(question dns.Question, answer *dns.Msg, response *dns.Msg) {
	response.Answer = append(response.Answer, answer.Answer...)
	response.Authoritative = true
	fmt.Printf("Query: %s, Reply: %s, Method: DNS server: %s\n", question.Name, answer.Answer[0].String(), answer.Answer[0].Header().Name[:len(answer.Answer[0].Header().Name)-1])

	// Cache the authoritative answers
	cacheRecords, err := getCacheRecords()
	if err != nil {
		log.Println("Error getting cache records:", err)
	}
	for _, authoritativeAnswer := range answer.Answer {
		cacheRecords = addToCache(cacheRecords, &authoritativeAnswer)
	}
	saveCacheRecords(cacheRecords)
}

func handleFallbackServer(question dns.Question, fallbackServer string, response *dns.Msg) {
	fallbackResponse, _ := queryAuthoritative(question.Name, fallbackServer)
	if fallbackResponse != nil {
		response.Answer = append(response.Answer, fallbackResponse.Answer...)
		fmt.Printf("Query: %s, Reply: %s, Method: Fallback DNS server: %s\n", question.Name, fallbackResponse.Answer[0].String(), fallbackServer)

		// Cache the fallback server answers
		cacheRecords, err := getCacheRecords()
		if err != nil {
			log.Println("Error getting cache records:", err)
		}
		for _, fallbackAnswer := range fallbackResponse.Answer {
			cacheRecords = addToCache(cacheRecords, &fallbackAnswer)
		}
		saveCacheRecords(cacheRecords)
	} else {
		fmt.Printf("Query: %s, No response\n", question.Name)
	}
}

func initializeJSONFiles() {
	createFileIfNotExists("servers.json", `{"servers": ["1.1.1.1:53", "1.0.0.1:53"]}`)
	createFileIfNotExists("records.json", `{"records": [{"Name": "example.com.", "Type": "A", "Value": "93.184.216.34", "TTL": 3600}]}`)
	createFileIfNotExists("cache.json", `{"records": []}`)
	createFileIfNotExists("dnsresolver.json", `{"fallback_server_ip": "192.168.178.21", "fallback_server_port": "53", "timeout": 2, "dns_port": "53", "cache_records": true, "auto_build_ptr_from_a": true}`)
}

func createFileIfNotExists(filename, content string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			log.Fatalf("Error creating %s: %s", filename, err)
		}
	}
}
