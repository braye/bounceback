package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	log.Println("bounceback 0.1.0")
	if len(os.Args) < 2 {
		fmt.Println("Missing server/client argument.")
		fmt.Println("Usage:")
		fmt.Println("bounceback server")
		fmt.Println("bounceback client example.com:31337")
		os.Exit(1)
	}

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

	for {
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
	if len(os.Args) < 3 {
		fmt.Println("Missing destination.")
		fmt.Println("Usage: bounceback client example.com:31337")
		os.Exit(1)
	}

	dest := os.Args[2]

	log.Println("Connecting..")

	var seq uint64
	pktHistory := make([]int64, 256)
	var rollingAverage int64

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

	throttlePosition, _ := time.ParseDuration("17ms")

	log.Printf("Connected to %s, gathering baseline...", destAddr)

	for {
		nextPktTime := time.Now().Add(throttlePosition)
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
		rtt := time.Since(sentTime)
		rttMicroseconds := int64(rtt.Microseconds())
		// log.Printf("Round trip took: %d microseconds.", rttMicroseconds)

		respInt := binary.LittleEndian.Uint64(resp)
		if err != nil {
			log.Println("Error parsing seq number from UDP response")
			log.Fatal(err)
		}

		if respInt != seq {
			log.Printf("DESYNC: expected seq number %d, received %d", seq, respInt)
			log.Println("DESYNC: sleeping 2 seconds")
			time.Sleep(2 * time.Second)
			continue
		}

		if seq > 256 && rollingAverage*10 <= rttMicroseconds {
			log.Printf("!!! Significant excursion from mean: %d microseconds. Seq %d", rttMicroseconds, seq)
		}

		pktHistory[seq%256] = rttMicroseconds

		// calculate our rolling average once the array is full, and then once every 32 packets
		if seq == 256 || seq >= 256 && seq%32 == 0 {
			rollingAverage = 0
			averageCount := int64(256)
			for k, t := range pktHistory {
				// discard outliers
				if k != 0 && pktHistory[k-1]*10 < t {
					averageCount--
					continue
				}
				rollingAverage += t
			}
			rollingAverage = rollingAverage / averageCount
			log.Printf("Rolling Average: %d microseconds", rollingAverage)
		}

		if time.Now().Before(nextPktTime) {
			time.Sleep(time.Until(nextPktTime))
		}
	}
}
