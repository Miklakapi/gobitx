# gobitx

`gobitx` is a small speed test tool written in Go.

It measures:

* latency
* download speed
* upload speed
* per-second transfer progress
* final transfer statistics

The control connection always uses TCP. Transfer tests use temporary TCP data connections.

## Usage

Start the server:

```bash
gobitx server
```

Run the client:

```bash
gobitx client <address>
```

Example:

```bash
gobitx client localhost
```

## Options

Available options for both `server` and `client`:

```text
-duration duration   Test duration, for example 10s, 30s or 1m
-port int            Port to listen on or connect to (default 5200)
-verbose             Show debug logs
```

Examples:

```bash
gobitx server -port 5200 -duration 10s
```

```bash
gobitx client -port 5200 -duration 10s localhost
```

## Output example

```text
Application started on port: :5200

Latency: samples=20 min=1.12ms avg=1.84ms max=3.41ms

Download: bytes=124.50 MiB duration=1.00s min=995.20 Mbps avg=995.20 Mbps max=995.20 Mbps stability=100.00%
Download: bytes=1.21 GiB duration=10.00s min=982.40 Mbps avg=1.04 Gbps max=1.08 Gbps stability=94.46%

Upload: bytes=118.80 MiB duration=1.00s min=950.40 Mbps avg=950.40 Mbps max=950.40 Mbps stability=100.00%
Upload: bytes=1.15 GiB duration=10.00s min=936.20 Mbps avg=988.70 Mbps max=1.02 Gbps stability=94.69%
```

## How it works

`gobitx` uses two types of TCP connections:

* **control connection** — used for commands, progress updates and final results
* **data connection** — temporary connection used only for raw transfer data

The server listens on the configured control port.

For every download or upload test, one side opens a temporary data port and sends that port to the other side through the control connection.

## Transfer direction

Download test:

```text
server -> client
```

The client receives data, calculates the download result and sends it back to the server.

Upload test:

```text
client -> server
```

The server receives data, calculates the upload result and sends it back to the client.

## Current status

Implemented:

* TCP control protocol
* latency test
* TCP download test
* TCP upload test
* transfer progress reporting
* final transfer statistics
