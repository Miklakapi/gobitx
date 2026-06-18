# gobitx

`gobitx` is a small speed test tool written in Go.

It measures:

* latency
* download speed
* upload speed
* UDP packet quality
* per-second progress
* final test statistics

The control connection always uses TCP. Download and upload tests use temporary TCP data connections. The quality test sends UDP packets and reports packet loss, jitter and received rate.

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
-duration duration   Duration of each test, for example 10s, 30s or 1m
-port int            Port to listen on or connect to (default 5200)
-verbose             Show debug logs
```

The duration value is applied separately to each test.

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

Quality: sent_packets=9991 received_packets=9991 lost_packets=0 loss_percent=0.00 avg_jitter=1.98µs max_jitter=54.00µs out_of_order=0 received_rate=95.87 Mbps
Quality: sent_packets=99884 received_packets=99884 lost_packets=0 loss_percent=0.00 avg_jitter=2.23µs max_jitter=647.00µs out_of_order=0 received_rate=95.88 Mbps
```

## How it works

`gobitx` uses a TCP control connection for commands, progress updates and final results.

For download and upload tests, it opens temporary TCP data connections used only for raw transfer data.

For the UDP quality test, the server opens a temporary UDP port and the client sends numbered UDP packets to it. The server calculates packet loss, jitter, out-of-order packets and received rate.

## Test direction

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

UDP quality test:

```text
client -> server
```

The server receives UDP packets, calculates quality statistics and sends the result back to the client.

## Current status

Implemented:

* TCP control protocol
* latency test
* TCP download test
* TCP upload test
* UDP quality test
* progress reporting
* final test statistics

