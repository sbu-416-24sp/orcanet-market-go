# Go Market Server

## Team Dolphins

An implementation of the OrcaNet market server, built using Go and [gRPC](https://grpc.io/docs/languages/go/quickstart).

## Setup (Continued from Sea Chicken)

1. Install [Go](https://go.dev/doc/install)
   * Ensure Go executables are available on PATH, (e.g. at `~/go/bin`)
2. Install protoc:

   `apt install protobuf-compiler`

   (May require more a [more recent version](https://grpc.io/docs/protoc-installation/#install-pre-compiled-binaries-any-os))
3. Install protoc-gen-go:

   `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`

4. Install protoc-gen-go-grpc:

   `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`


## Running the Program

To start the host Node:

```Shell
go run server/main.go
```

To run another node and connect, paste the multiaddress provided from the previous command (it should be next to "Listen address: ". Use the one that is not 127.0.0.1 as other nodes will not recognize the local IP address)

```Shell
go run server/main.go -bootstrap (multiaddress)
```

To compile the protobuf at `market/market.proto`:

```Shell
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  market/market.proto
```

## API
Detailed gRPC endpoints are in `market/market.proto`

- Holders of a file can register the file using the RegisterFile RPC.
  - Provide a User with 5 fields: 
    - `id`: some string to identify the user.
    - `name`: a human-readable string to identify the user
    - `ip`: a string of the public ip address
    - `port`: an int32 of the port
    - `price`: an int64 that details the price per mb of outgoing files
  - Provide a fileHash string that is the hash of the file
  - Returns nothing

- Then, clients can search for holders using the CheckHolders RPC
  - Provide a fileHash to identify the file to search for
  - Returns a list of Users that hold the file.

## Functionality
Once connected, the program will ask to enter a username and a price for supplying files. After more than one peer is detected, peers can perform the following options:

1. Register a file
Enter the file hash when prompted. This file hash will be added to the DHT.
2. Check holders for a file
Enter the hash of file you want. This will return a list of all holders of that file.
3. Check for connected peers
Returns a list of all peers that are currently connected and disconnected to the DHT.
4. Exit
Disconnects node from the network.