# bounceback

This repository implements a simple network diagnostic tool which passes UDP packets across the network to a server, and measures round-trip-time.

Useful for capturing intermittent jitter, e.g. on wireless network connections.

## tl;dr

```
Usage:
(Server) bounceback [--port 31337]
(Client) bounceback --host example.com [--port 31337] [--rate 60]
```

## Building

The tool has no external dependencies - simply run `go build .` in the project repository after cloning. Requires at least go 1.13.

## Usage

The tool has a simple client-server architecture. Run `bounceback` on the endpoint you wish to measure latency to. The server listens for connections from the client on port 31337 by default. Use the `--port` arg to change this.

Next, launch the client on the endpoint you wish to measure latency from, passing the hostname of the server with the `--host` argument. The client will begin passing packets to the server, and measuring the round-trip-time. It will report any significant excursions from mean, defined as 10x the rolling average of the last 256 packets. These outliers are automatically removed from the rolling average.

There is throttling in place, such that the client will never send more than 60 packets/sec. This can be overriden with the `--rate` flag, which specifies the number of packets/second to send.