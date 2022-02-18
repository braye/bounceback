package main

import (
	"net"
	"log"
	"os"
	"time"
	"encoding/binary"
)

func main() {
	if os.Args[1] == "client" {
		bouncebackClient()
	} else {
		bouncebackServer()
	}
}

func bouncebackServer() {
	listenerUDPAddr, err := net.ResolveUDPAddr("udp4", ":31337")
	if err != nil {
		log.Println("error resolving listen address:")
		log.Fatal(err)
	}

	metrics, err := net.ListenUDP("udp4", listenerUDPAddr)
	if err != nil {
		log.Println("Error listening on port 31337.")
		log.Fatal(err)
	}
	log.Println("Listening on port 31337.")
	defer metrics.Close()

	for{
		msg := make([]byte, 8)
		_, addr, err := metrics.ReadFrom(msg)
		if err != nil {
			log.Printf("Error reading msg from udp connection: %s", err)
			break
		}
		log.Printf("recieved packet from %s, seq %d", addr, binary.LittleEndian.Uint64(msg))
		_, err = metrics.WriteTo(msg, addr)
	}
}

func bouncebackClient() {
	dest := os.Args[2]

	log.Println("Connecting..")

	var seq uint64

	destAddr, err := net.ResolveUDPAddr("udp4", dest)
	if err != nil {
		log.Println("error resolving destination address:")
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp4", nil, destAddr)
	if err != nil {
		log.Println("Error connecting to destination:")
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		// TODO: add throttling
		// nextPktTime := time.Now().Add(time.ParseDuration("17ms"))
		seq++
		msg := make([]byte, 8)
		resp := make([]byte, 8)
		binary.LittleEndian.PutUint64(msg, seq)
		_, err = conn.Write(msg)

		sentTime := time.Now()
		if err != nil {
			log.Println(err)
			log.Println("Sleeping for 5s and retrying...")
			time.Sleep(5 * time.Second)
			continue
		}
		_, err = conn.Read(resp)
		duration := time.Since(sentTime)

		respInt := binary.LittleEndian.Uint64(resp)
		if err != nil {
			log.Println("Error parsing seq number from UDP response")
			log.Fatal(err)
		}

		if respInt != seq {
			log.Println("DESYNC: sleeping 2 seconds")
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("Round trip took: %d microseconds.", duration.Microseconds())
	}
}