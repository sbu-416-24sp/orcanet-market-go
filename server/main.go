/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Modified by Stony Brook University students
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 *
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"

	pb "orcanet/market"
)

var (
	clientMode           = flag.Bool("client", false, "run this program in client mode")
	startBootstrapNodeAt = flag.String("startBootstrapNodeAt", "", "start a bootstrap node")
	bootstrap            = flag.String("bootstrap", "", "multiaddresses to bootstrap to")
	addr                 = flag.String("addr", "", "multiaddresses to listen to")
)

type CustomValidator struct{}

func (cv CustomValidator) Select(string, [][]byte) (int, error) {
	return 0, nil
}

func (cv CustomValidator) Validate(key string, value []byte) error {
	return nil
}

func main() {
	ctx := context.Background()
	flag.Parse()

	// Convert the strings (from flags) to multiaddresses
	listenAddr, _ := multiaddr.NewMultiaddr(*addr)
	// startBootstrapAddr, _ := multiaddr.NewMultiaddr(*startBootstrapNodeAt)

	var bootstrapPeers []multiaddr.Multiaddr

	// Check if a bootstrap address is provided.
	if len(*bootstrap) > 0 {
		// Try to create a multiaddr from the provided bootstrap flag.
		bootstrapAddr, err := multiaddr.NewMultiaddr(*bootstrap)
		if err != nil {
			// Handle the error properly (e.g., log it or exit).
			fmt.Errorf("Invalid bootstrap address: %s", err)
		}
		// Use the provided bootstrap address.
		bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
	} else {
		// Use default bootstrap peers if no address is provided.
		bootstrapPeers = dht.DefaultBootstrapPeers
	}

	// Initialize a new libp2p Host
	host, err := libp2p.New(libp2p.ListenAddrs(listenAddr))
	if err != nil {
		fmt.Errorf("Failed to create host: %s", err)
	}
	fmt.Println("Host created with ID ", host.ID())

	// Initialize the DHT
	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		fmt.Errorf("Failed to create DHT: %s", err)
	}
	fmt.Println("DHT created")

	// Bootstrap the DHT
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		fmt.Errorf("Failed to bootstrap DHT: %s", err)
	}

	// Connect the validator
	kademliaDHT.Validator = &CustomValidator{}

	connectToBootstrapPeers(ctx, host, bootstrapPeers)

	// Prompt for username in terminal
	var username string
	fmt.Print("Enter username: ")
	fmt.Scanln(&username)

	// Generate a random ID for new user
	userID := fmt.Sprintf("user%d", rand.Intn(10000))

	fmt.Print("Enter a price for supplying files: ")
	var price int64
	_, err = fmt.Scanln(&price)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	// Create a User struct with the provided username and generated ID
	user := &pb.User{
		Id:    userID,
		Name:  username,
		Ip:    "localhost",
		Port:  416320,
		Price: price,
	}

	for {
		go peerDiscovery(ctx, host, kademliaDHT)

		fmt.Println("---------------------------------")
		fmt.Println("1. Register a file")
		fmt.Println("2. Check holders for a file")
		fmt.Println("3. Exit")
		fmt.Print("Option: ")
		var choice int
		_, err := fmt.Scanln(&choice)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		if choice == 3 {
			return
		}

		fmt.Print("Enter a file hash: ")
		var fileHash string
		_, err = fmt.Scanln(&fileHash)
		if err != nil {
			fmt.Errorf("Error: ", err)
			continue
		}

		switch choice {
		case 1:
			req := &pb.RegisterFileRequest{User: user, FileHash: fileHash}
			registerFile(ctx, kademliaDHT, req)
		case 2:
			checkReq := &pb.CheckHoldersRequest{FileHash: fileHash}
			holdersResp, _ := checkHolders(ctx, kademliaDHT, checkReq)
			fmt.Println("Holders:", holdersResp.Holders)
		case 3:
			return
		default:
			fmt.Println("Unknown option: ", choice)
		}

		fmt.Println()
	}
}

func connectToBootstrapPeers(ctx context.Context, host host.Host, bootstrapPeers []multiaddr.Multiaddr) {
	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				fmt.Errorf("Failed to connect to bootstrap peer %v: %s", peerinfo, err)
			} else {
				fmt.Printf("Connected to bootstrap peer: %s\n", peerinfo.ID)
			}
		}()
	}
	wg.Wait()
}

func peerDiscovery(ctx context.Context, host host.Host, dht *dht.IpfsDHT) {
	routingDiscovery := drouting.NewRoutingDiscovery(dht)
	dutil.Advertise(ctx, routingDiscovery, "market")
	peerChan, _ := routingDiscovery.FindPeers(ctx, "market")

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}
		fmt.Println("Found peer:", peer)
	}
}

// register that the a user holds a file, then add the user to the list of file holders
func registerFile(ctx context.Context, dht *dht.IpfsDHT, req *pb.RegisterFileRequest) error {
	//serialize the User object to byte slice for storage
	data, err := proto.Marshal(req.User)
	if err != nil {
		errMsg := fmt.Sprintf("Error marshaling user data for file hash %s: %v", req.FileHash, err)
		fmt.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	// Use the file hash as the key to store serialized user data in the DHT
	// for now use file hash provided by user in Req object
	key := fmt.Sprintf("/market/file/%s", req.FileHash)
	if err := dht.PutValue(ctx, key, data); err != nil {
		errMsg := fmt.Sprintf("Error putting value in the DHT for file hash %s: %v", req.FileHash, err)
		fmt.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	fmt.Printf("Successfully registered file with hash %s\n", req.FileHash)
	return nil
}

// CheckHolders returns a list of user names holding a file with a hash
func checkHolders(ctx context.Context, dht *dht.IpfsDHT, req *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {
	key := fmt.Sprintf("/market/file/%s", req.FileHash)
	data, err := dht.GetValue(ctx, key)
	if err != nil {
		fmt.Printf("Failed to get value from the DHT: %v", err)
		return nil, err
	}

	// Deserialize the data back into a User struct
	var user pb.User
	if err := proto.Unmarshal(data, &user); err != nil {
		fmt.Printf("Failed to unmarshal user data: %v", err)
		return nil, err
	}

	// Wrap the User in a HoldersResponse and return
	return &pb.HoldersResponse{Holders: []*pb.User{&user}}, nil
}
