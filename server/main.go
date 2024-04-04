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
	"time"

	pb "orcanet/market"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
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
	var listenAddrString string
	if *addr == "" {
		listenAddrString = "/ip4/0.0.0.0/tcp/0" // Default value
	} else {
		listenAddrString = *addr
	}
	listenAddr, _ := multiaddr.NewMultiaddr(listenAddrString)

	var bootstrapPeers []multiaddr.Multiaddr

	// Check if a bootstrap address is provided.
	if len(*bootstrap) > 0 {
		// Try to create a multiaddr from the provided bootstrap flag.
		bootstrapAddr, err := multiaddr.NewMultiaddr(*bootstrap)
		if err != nil {
			fmt.Errorf("Invalid bootstrap address: %s", err)
		}
		// Use the provided bootstrap address.
		bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
	} else {
		// Use default bootstrap peers if no address is provided.
		//bootstrapPeers = dht.DefaultBootstrapPeers
	}

	host, err := libp2p.New(libp2p.ListenAddrs(listenAddr))
	if err != nil {
		fmt.Errorf("Failed to create host: %s", err)
	}
	fmt.Println("Host created with ID ", host.ID())
	for _, addr := range host.Addrs() {
		// The "/ipfs/" prefix is used here for historical reasons.
		// It may be "/p2p/" in other contexts or newer versions.
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr, host.ID())
		fmt.Println("Listen address:", fullAddr)
	}

	// Initialize the DHT
	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		fmt.Errorf("Failed to create DHT: %s", err)
		return
	}
	fmt.Println("DHT created")

	// Connect the validator
	kademliaDHT.Validator = &CustomValidator{}

	// Bootstrap the DHT
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		fmt.Errorf("Failed to bootstrap DHT: %s", err)
	}

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

	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)

	fmt.Println("Looking for existence of peers on the network before proceeding...")
	checkPeerExistence(ctx, host, kademliaDHT, routingDiscovery)
	fmt.Println("Peer(s) found! proceeding with the application.")

	for {
		go peerDiscovery(ctx, host, kademliaDHT, routingDiscovery)

		fmt.Println("---------------------------------")
		fmt.Println("1. Register a file")
		fmt.Println("2. Check holders for a file")
		fmt.Println("3. Check for connected peers")
		fmt.Println("4. Exit")
		fmt.Print("Option: ")
		var choice int
		_, err := fmt.Scanln(&choice)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		if choice == 4 {
			return
		}

		switch choice {
		case 1:
			fmt.Print("Enter a file hash: ")
			var fileHash string
			_, err = fmt.Scanln(&fileHash)
			if err != nil {
				fmt.Errorf("Error: ", err)
				continue
			}
			req := &pb.RegisterFileRequest{User: user, FileHash: fileHash}
			registerFile(ctx, kademliaDHT, req)
		case 2:
			fmt.Print("Enter a file hash: ")
			var fileHash string
			_, err = fmt.Scanln(&fileHash)
			if err != nil {
				fmt.Errorf("Error: ", err)
				continue
			}
			checkReq := &pb.CheckHoldersRequest{FileHash: fileHash}
			holdersResp, _ := checkHolders(ctx, kademliaDHT, checkReq)
			fmt.Println("Holders:")
			for _, holder := range holdersResp.Holders {
				fmt.Println(holder)
			}
			//fmt.Println("Holders:", holdersResp.Holders)
		case 3:
			printRoutingTable(kademliaDHT)
		case 4:
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

func checkPeerExistence(ctx context.Context, host host.Host, dht *dht.IpfsDHT, routingDiscovery *drouting.RoutingDiscovery) bool {
	if len(dht.RoutingTable().ListPeers()) > 0 {
		return true
	}

	for {
		isPeersFound := peerDiscovery(ctx, host, dht, routingDiscovery)
		if isPeersFound {
			return true
		}
		fmt.Println("No peers found, waiting...")
		time.Sleep(7 * time.Second) // Wait for 5 seconds before trying again
	}
}

func peerDiscovery(ctx context.Context, host host.Host, dht *dht.IpfsDHT, routingDiscovery *drouting.RoutingDiscovery) bool {
	dutil.Advertise(ctx, routingDiscovery, "market")

	peerChan, err := routingDiscovery.FindPeers(ctx, "market")
	if err != nil {
		fmt.Println("Failed to find peers:", err)
		return false
	}

	peerDiscovered := false
	for peer := range peerChan {
		if peer.ID == host.ID() {
			fmt.Printf("Connected to: %s (Myself) \n", peer.ID)
			continue
		}
		err := host.Connect(ctx, peer)
		if err != nil {
			fmt.Printf("Failed connecting to %s, error: %s\n", peer.ID, err)
		} else {
			fmt.Printf("Connected to: %s\n", peer.ID)
			for _, addr := range peer.Addrs {
				fmt.Printf("Address: %s\n", addr)
			}
			peerDiscovered = true
		}
	}
	return peerDiscovered
}

func printRoutingTable(dht *dht.IpfsDHT) {
	for _, peer := range dht.RoutingTable().ListPeers() {
		fmt.Println("Peer ID:", peer)
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

	key := fmt.Sprintf("/market/file/%s/%s", req.FileHash, dht.PeerID())
	print(key)

	if err := dht.PutValue(ctx, key, data); err != nil {
		errMsg := fmt.Sprintf("Error putting value in the DHT for file hash %s: %v", req.FileHash, err)
		fmt.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	fmt.Printf("Successfully registered file with hash %s\n", req.FileHash)
	return nil
}

func checkHolders(ctx context.Context, dht *dht.IpfsDHT, req *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {

	var holders []*pb.User
	fmt.Println("Searching for " + req.FileHash)
	var allPeers []peer.ID = dht.RoutingTable().ListPeers()
	allPeers = append(allPeers, dht.PeerID())

	// iterate through each peer to see if they own the file
	for _, peer := range allPeers {

		key := fmt.Sprintf("/market/file/%s/%s", req.FileHash, peer)
		dataChan, err := dht.SearchValue(ctx, key)
		if err != nil {
			fmt.Printf("Failed to get value from the DHT: %v", err)
			continue
		}

		var forLoopRunning = true // break out of the for loop once the channel has been closed
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					// Channel has been closed, we've received all the data
					//return &pb.HoldersResponse{Holders: holders}, nil
					forLoopRunning = false
					break // this breaks out of the select loop
				}
				// Deserialize the data back into a User struct
				var user pb.User
				if err := proto.Unmarshal(data, &user); err != nil {
					fmt.Printf("Failed to unmarshal user data: %v", err)
					continue // Skip this iteration
				}
				holders = append(holders, &user)
			case <-ctx.Done():
				// The context was cancelled or expired
				fmt.Println("Context cancelled or expired.")
				return nil, ctx.Err()
			}
			// break out of for loop when forLoopRunning is set to false
			if !forLoopRunning {
				break
			}
		}

	}

	return &pb.HoldersResponse{Holders: holders}, nil

}
