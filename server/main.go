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
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"

	pb "orcanet/market"
)

/*

var (
	port = flag.Int("port", 50051, "The server port")
)

// map file hashes to supplied files + prices
var files = make(map[string][]*pb.RegisterFileRequest)

// print the current holders map
func printHoldersMap() {
	for hash, holders := range files {
		fmt.Printf("\nFile Hash: %s\n", hash)

		for _, holder := range holders {
			user := holder.GetUser()
			fmt.Printf("Username: %s, Price: %d\n", user.GetName(), user.GetPrice())
		}

	}
}

type server struct {
	pb.UnimplementedMarketServer
}

*/

// DefaultBootstrapPeers is a set of public DHT bootstrap peers provided by libp2p.
// obtained here https://github.com/libp2p/go-libp2p-kad-dht/blob/master/dht_bootstrap.go
var DefaultBootstrapPeers = []string{
    "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
    "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
    "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
    "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
    "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ", 
}


// GetDefaultBootstrapPeerAddrInfos parses the bootstrap peers' multiaddresses
// into peer.AddrInfo objects for libp2p networking.
func GetDefaultBootstrapPeerAddrInfos() ([]peer.AddrInfo, error) {
    var peers []peer.AddrInfo
    for _, addrStr := range DefaultBootstrapPeers {

		// Convert the string to a multiaddr.Multiaddr object
        addr, err := multiaddr.NewMultiaddr(addrStr)
        if err != nil {
            return nil, err
        }

		//construct a peer.AddrInfo object from the Multiaddr above 
        pi, err := peer.AddrInfoFromP2pAddr(addr)
        if err != nil {
            return nil, err
        }

		//append peers
        peers = append(peers, *pi)
    }

	//returns list of AddrInfo objects 
    return peers, nil
}


//function to iterate over addr info objects and connect 
func connectToBootstrapPeers(ctx context.Context, h host.Host, bootstrapPeers []peer.AddrInfo) {
    for _, peerInfo := range bootstrapPeers {
		//attempt to connect
        if err := h.Connect(ctx, peerInfo); err != nil {
            log.Printf("Failed to connect to bootstrap peer %v: %s", peerInfo, err)
        } else {
            fmt.Printf("Connected to bootstrap peer: %s\n", peerInfo.ID)
        }
    }
}



// each key needs to validated before being put in the DHT. For now we accept all hashes
// https://github.com/libp2p/specs/tree/master/kad-dht#entry-validation

// create custom validator function
type CustomValidator struct{}

//validator for key
func (cv CustomValidator) Select( string,  [][]byte) (int, error) {
	
	//accept all for now
    return 0, nil 
}

// validates whether a value is acceptable or not
func (cv CustomValidator) Validate(key string, value []byte) error {
    
	//accept all for now 
    return nil
}


func main() {
    ctx := context.Background()

    // Initialize a new libp2p Host
    host, err := libp2p.New()
    if err != nil {
        log.Fatalf("Failed to create host: %s", err)
    }

    // Initialize the DHT with the host
    kademliaDHT, err := dht.New(ctx, host)
    if err != nil {
        log.Fatalf("Failed to create DHT: %s", err)
    }

    // Bootstrap the DHT
    if err := kademliaDHT.Bootstrap(ctx); err != nil {
        log.Fatalf("Failed to bootstrap DHT: %s", err)
    }

	// Connect the validator
	kademliaDHT.Validator = &CustomValidator{}

    // Retrieve and connect to default bootstrap peers (hardcoded list above)
    bootstrapPeers, err := GetDefaultBootstrapPeerAddrInfos()
    if err != nil {
        log.Fatalf("Failed to get default bootstrap peers: %s", err)
    }
    connectToBootstrapPeers(ctx, host, bootstrapPeers)

	//print our host id
    fmt.Println("Host created. Host ID:", host.ID())
	
	// test example
    req := &pb.RegisterFileRequest{
        User: &pb.User{
            Id:    "user1",
            Name:  "Jackie",
            Ip:    "127.0.0.1",
            Port:  4001,
            Price: 100,
        },
        FileHash: "testhash",
    }
    if err := registerFileToDHT(context.Background(), kademliaDHT, req); err != nil {
        log.Fatal(err)
    }
    if err := registerFileToDHT(context.Background(), kademliaDHT, req); err != nil {
        log.Fatal(err)
    }

    // Check for holders
    checkReq := &pb.CheckHoldersRequest{FileHash: "testhash"}
    holdersResp, err := checkHoldersFromDHT(context.Background(), kademliaDHT, checkReq)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Holders:", holdersResp.Holders)

}

// register that the a user holds a file, then add the user to the list of file holders
func registerFileToDHT(ctx context.Context, dht *dht.IpfsDHT, req *pb.RegisterFileRequest) error {
	//serialize the User object to byte slice for storage
    data, err := proto.Marshal(req.User)
    if err != nil {
        errMsg := fmt.Sprintf("Error marshaling user data for file hash %s: %v", req.FileHash, err)
        log.Println(errMsg)
        return fmt.Errorf(errMsg) 
    }

	
    // Use the file hash as the key to store serialized user data in the DHT
	// for now use file hash provided by user in Req object
    key := fmt.Sprintf("/market/file/%s", req.FileHash)
    if err := dht.PutValue(ctx, key, data); err != nil {
        errMsg := fmt.Sprintf("Error putting value in the DHT for file hash %s: %v", req.FileHash, err)
        log.Println(errMsg)
        return fmt.Errorf(errMsg) 
    }

    fmt.Printf("Successfully registered file with hash %s\n", req.FileHash)
    return nil
}



// CheckHolders returns a list of user names holding a file with a hash
func checkHoldersFromDHT(ctx context.Context, dht *dht.IpfsDHT, req *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {
    key := fmt.Sprintf("/market/file/%s", req.FileHash)
    data, err := dht.GetValue(ctx, key)
    if err != nil {
        log.Printf("Failed to get value from the DHT: %v", err)
        return nil, err
    }

    // Deserialize the data back into a User struct
    var user pb.User
    if err := proto.Unmarshal(data, &user); err != nil {
        log.Printf("Failed to unmarshal user data: %v", err)
        return nil, err
    }

    // Wrap the User in a HoldersResponse and return
    return &pb.HoldersResponse{Holders: []*pb.User{&user}}, nil
}



/*

// register that the a user holds a file, then add the user to the list of file holders
func (s *server) RegisterFile(ctx context.Context, in *pb.RegisterFileRequest) (*emptypb.Empty, error) {
	hash := in.GetFileHash()

	files[hash] = append(files[hash], in)
	fmt.Printf("Num of registered files: %d\n", len(files[hash]))
	return &emptypb.Empty{}, nil
}

// CheckHolders returns a list of user names holding a file with a hash
func (s *server) CheckHolders(ctx context.Context, in *pb.CheckHoldersRequest) (*pb.HoldersResponse, error) {
	hash := in.GetFileHash()

	holders := files[hash]

	users := make([]*pb.User, len(holders))
	for i, holder := range holders {
		users[i] = holder.GetUser()
	}

	printHoldersMap()

	return &pb.HoldersResponse{Holders: users}, nil
}


*/