# bounceback

This repository implements a simple network diagnostic tool which passes UDP packets across the network to a server, and measures round-trip-time.

Useful for capturing intermittent jitter, e.g. on wireless network connections.

## Building

The tool has no external dependencies - simply run `go build .` in the project repository after cloning. Built on go 1.16, also tested on 1.15.

## Usage

The tool has a simple client-server architecture. Run `bounceback server` on the endpoint you wish to measure latency to. The server listens for connections from the client on port 31337, on all IPv4 addresses.

Next, launch the client on the endpoint you wish to measure latency *from*, by running `bounceback client [your server addr]:31337`. The client will begin passing packets to the server, and measuring the round-trip-time. It will report any significant excursions from mean, defined as 10x the rolling average of the last 256 packets. These outliers are automatically removed from the rolling average.

There is throttling in place, such that the client will never send more than ~60 packets/sec.