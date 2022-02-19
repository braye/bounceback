package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	fmt.Println("bounceback 0.2.1")

	mode := "server"
	port := 31337
	rate := 60
	host := "example.com"

	for k, arg := range os.Args {
		switch arg {
		case "--host":
			if len(os.Args) < k+2 {
				fmt.Println("Missing value for host argument.")
				usageMsg()
			}
			host = os.Args[k+1]
			mode = "client"
		case "--port":
			if len(os.Args) < k+2 {
				fmt.Println("Missing value for port argument.")
				usageMsg()
			}
			p, err := strconv.Atoi(os.Args[k+1])
			if err != nil {
				fmt.Println("Could not parse port argument as int.")
				usageMsg()
			}
			port = p
		case "--rate":
			// TODO: code reuse, meh
			if len(os.Args) < k+2 {
				fmt.Println("Missing value for rate argument.")
				usageMsg()
			}
			r, err := strconv.Atoi(os.Args[k+1])
			if err != nil {
				fmt.Println("Could not parse rate argument as int.")
				usageMsg()
			}
			rate = r
		}
	}

	if mode == "client" {
		bouncebackClient(host, port, rate)
	} else {
		bouncebackServer(port)
	}
}

func usageMsg() {
	fmt.Println("Usage:")
	fmt.Println("(Server) bounceback [--port 31337]")
	fmt.Println("(Client) bounceback --host example.com [--port 31337] [--rate 60]")
	os.Exit(1)
}

func bouncebackServer(port int) {
	listenAddr := fmt.Sprintf(":%d", port)
	listenerUDPAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Println("Error resolving listen address:")
		log.Fatal(err)
	}

	metrics, err := net.ListenUDP("udp", listenerUDPAddr)
	if err != nil {
		log.Printf("Error listening on port %d.", port)
		log.Fatal(err)
	}
	log.Printf("Listening on port %d.", port)
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

func bouncebackClient(host string, port int, rate int) {
	dest := fmt.Sprintf("%s:%d", host, port)
	destAddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		log.Println("Error resolving destination address:")
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, destAddr)
	if err != nil {
		log.Println("Error connecting to destination:")
		log.Fatal(err)
	}
	defer conn.Close()

	hertzToMs := 1000/rate

	throttlePosition, err := time.ParseDuration(fmt.Sprintf("%dms",hertzToMs))
	if err != nil {
		log.Println("Could not parse rate argument.")
		log.Fatal(err)
	}

	timeout, _ := time.ParseDuration("1000ms")

	log.Printf("Connected to %s, gathering baseline...", destAddr)

	// int64s because why not
	var seq uint64
	pktHistory := make([]int64, 256)
	var rollingAverage int64

	// main packet sending loop
	for {
		nextPktTime := time.Now().Add(throttlePosition)
		timeoutTime := time.Now().Add(timeout)
		conn.SetReadDeadline(timeoutTime)
		seq++
		msg := make([]byte, 8)
		resp := make([]byte, 8)
		// our UDP packet consists of a single uint64, representing packet number
		binary.LittleEndian.PutUint64(msg, seq)
		_, err = conn.Write(msg)

		// only start timing after conn.Write returns, to mitigate os/driver latency
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
		if err != nil {
			log.Println(err)
			log.Println("Error reading UDP packet, sleeping 5 seconds and retrying.")
			time.Sleep(5 * time.Second)
			continue
		}

		respInt := binary.LittleEndian.Uint64(resp)

		// if we get an out-of-order response for some reason, let the network settle and try again
		// also trips if our response is mangled in some way
		if respInt != seq {
			log.Printf("DESYNC: expected seq number %d, received %d", seq, respInt)
			log.Println("DESYNC: sleeping 2 seconds")
			time.Sleep(2 * time.Second)
			continue
		}

		// define "significant" jitter as 10x the rolling average
		// not all UDP applications are sensitive to this level of jitter
		// VOIP and delay-based netcode games (path of exile, most JP fighting games) do not tolerate this level of jitter well
		if seq > 256 && rollingAverage*10 <= rttMicroseconds {
			log.Printf("!!! Significant excursion from mean: %d microseconds. Seq %d", rttMicroseconds, seq)
		}

		pktHistory[seq%256] = rttMicroseconds

		// calculate our rolling average once the array is full, and then once every 32 packets
		if seq == 256 || seq >= 256 && seq%32 == 0 {
			rollingAverage = 0
			averageCount := int64(256)
			for k, t := range pktHistory {
				// discard outliers to keep our sense of the "normal" network state
				if k != 0 && pktHistory[k-1]*10 < t {
					averageCount--
					continue
				}
				rollingAverage += t
			}
			rollingAverage = rollingAverage / averageCount
			log.Printf("Rolling Average: %d microseconds", rollingAverage)
		}

		// throttle requests so we don't induce network problems by effectively DoSing
		if time.Now().Before(nextPktTime) {
			time.Sleep(time.Until(nextPktTime))
		}
	}
}
