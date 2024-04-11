package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"

	pb "orcanet/market"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

var (
	clientMode = flag.Bool("client", false, "run this program in client mode")
	bootstrap  = flag.String("bootstrap", "", "multiaddresses to bootstrap to")
	addr       = flag.String("addr", "", "multiaddresses to listen to")
)

type DHTvalue struct {
	Ids    string
	Names  string
	Ips    string
	Ports  string
	Prices string
}

type CustomValidator struct{}

func (cv CustomValidator) Select(string, [][]byte) (int, error) {
	return 0, nil
}

func (cv CustomValidator) Validate(key string, value []byte) error {
	//  length := 10
	//  hexRegex := regexp.MustCompile("^[0-9a-fA-F]{" + strconv.Itoa(length) + "}$")
	//  if !hexRegex.MatchString(key) {
	// 	 return errors.New("input is not a valid hexadecimal string or does not match the expected length")
	//  }
	return nil
}

func main() {
	ctx := context.Background()
	flag.Parse()

	// Construct listening address
	var listenAddrString string
	if *addr == "" {
		listenAddrString = "/ip4/0.0.0.0/tcp/0"
	} else {
		listenAddrString = *addr
	}
	listenAddr, _ := multiaddr.NewMultiaddr(listenAddrString)

	var bootstrapPeers []multiaddr.Multiaddr

	// Connect to bootstrap peers
	if len(*bootstrap) > 0 {
		bootstrapAddr, err := multiaddr.NewMultiaddr(*bootstrap)
		if err != nil {
			fmt.Errorf("Invalid bootstrap address: %s", err)
		}
		bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
	}

	// Create host
	privKey, err := LoadOrCreateKey("./peer_identity.key")
	if err != nil {
		panic(err)
	}

	host, err := libp2p.New(libp2p.ListenAddrs(listenAddr), libp2p.Identity(privKey))
	if err != nil {
		fmt.Errorf("Failed to create host: %s", err)
	}

	for _, addr := range host.Addrs() {
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr, host.ID())
		fmt.Println("Listen address:", fullAddr)
	}
	// Initialize the DHT
	var kademliaDHT *dht.IpfsDHT
	if *clientMode {
		kademliaDHT, err = dht.New(ctx, host, dht.Mode(dht.ModeClient))
	} else {
		kademliaDHT, err = dht.New(ctx, host, dht.Mode(dht.ModeServer))
	}
	if err != nil {
		fmt.Errorf("Failed to create DHT: %s", err)
		return
	}

	// Connect the validator
	kademliaDHT.Validator = &CustomValidator{}

	// Bootstrap the DHT
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		fmt.Errorf("Failed to bootstrap DHT: %s", err)
	}

	connectToBootstrapPeers(ctx, host, bootstrapPeers)

	// Create the user, connect to peers and run CLI
	user := promptForUserInfo(ctx)
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)

	fmt.Println("Looking for existence of peers on the network before proceeding...")
	checkPeerExistence(ctx, host, kademliaDHT, routingDiscovery)
	fmt.Println("Peer(s) found! proceeding with the application.")

	runCLI(ctx, host, kademliaDHT, routingDiscovery, user)
}

// promptForUserInfo creates a user struct based on information
// entered by the user in the terminal
//
// Parameters:
// - ctx: A context.Context for controlling the function's execution lifetime.
//
// Returns: A *pb.User containing the ID, name, IP address, port, and price of the user
func promptForUserInfo(ctx context.Context) *pb.User {
	var username string
	fmt.Print("Enter username: ")
	fmt.Scanln(&username)

	// Generate a random ID for new user
	userID := fmt.Sprintf("user%d", rand.Intn(10000))

	fmt.Print("Enter a price for supplying files: ")
	var price int64
	fmt.Scanln(&price)

	user := &pb.User{
		Id:    userID,
		Name:  username,
		Ip:    "localhost",
		Port:  416320,
		Price: price,
	}

	return user
}

// runCLI provides a command line interface for users to register files, check the holders for a file,
// and check for connected peers
//
// Parameters:
// - ctx: A context.Context for controlling the function's execution lifetime.
// - host: The local host.Host instance.
// - dht: A pointer to the dht.IpfsDHT instance used for direct DHT operations.
// - routingDiscovery: A pointer to the drouting.RoutingDiscovery used for peer discovery.
//
// Returns: None
func runCLI(ctx context.Context, host host.Host, dht *dht.IpfsDHT, routingDiscovery *drouting.RoutingDiscovery, user *pb.User) {
	for {
		go peerDiscovery(ctx, host, dht, routingDiscovery)

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
			registerFile(ctx, dht, req)
		case 2:
			fmt.Print("Enter a file hash: ")
			var fileHash string
			_, err = fmt.Scanln(&fileHash)
			if err != nil {
				fmt.Errorf("Error: ", err)
				continue
			}
			checkReq := &pb.CheckHoldersRequest{FileHash: fileHash}
			holdersResp, _ := checkHolders(ctx, dht, checkReq)
			for _, user := range holdersResp.Holders {
				fmt.Println("User:", user)
			}
		case 3:
			printRoutingTable(dht)
		case 4:
			return
		default:
			fmt.Println("Unknown option: ", choice)
		}

		fmt.Println()
	}
}
