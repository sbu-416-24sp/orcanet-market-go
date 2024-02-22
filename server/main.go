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
	"log"
	"net"
	"strings"

	pb "orcanet/market"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

// maps a file to a list of users who want this file
var fileRequesters = make(map[string][]*pb.User)

// map of files to users holding the file
var fileHolders = make(map[string][]*pb.User)

// print the current requesters map 
func printRequestersMap() {
    for fileID, users := range fileRequesters {
        fmt.Print("\nFile ID: ", fileID, "\nUsers Requesting File: \n")
        userNames := []string{}
        for _, user := range users {
            userNames = append(userNames, user.GetName())
        }
        fmt.Println(strings.Join(userNames, "\n"))
    }
}

// print the current holders map
func printHoldersMap() {
    for fileID, holders := range fileHolders {
        fmt.Print("\nFile ID: ", fileID, "\nUsers Holding File: \n")
        holderNames := []string{}
        for _, holder := range holders {
            holderNames = append(holderNames, holder.GetName())
        }
        fmt.Println(strings.Join(holderNames, "\n"))
    }
}


type server struct {
	pb.UnimplementedMarketServer
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterMarketServer(s, &server{})
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error %v", err)
	}
}

// Add a request that a user with userId wants file with fileId
func (s *server) RequestFile(ctx context.Context, in *pb.FileRequest) (*pb.FileResponse, error) {
	user := in.GetUser()
	fileId := in.GetFileId()

	// Check if file is held by anyone; I hate Go
	if _, ok := fileHolders[fileId]; !ok {
		return &pb.FileResponse{Exists: false, Message: "File not found"}, nil
	}

	fileRequesters[fileId] = append(fileRequesters[fileId], user)

	return &pb.FileResponse{Exists: true, Message: "OK"}, nil
}

// Get a list of userIds who are requesting a file with fileId
func (s *server) CheckRequests(ctx context.Context, in *pb.CheckRequest) (*pb.ListReply, error) {
	fileId := in.GetFileId()

	users := fileRequesters[fileId]

	// Make list of userIDs from the users struct and return
	userNames := make([]string, len(users))
	for i, user := range users {
		userNames[i] = user.GetName()
	}

	printRequestersMap()

	return &pb.ListReply{Strings: userNames}, nil
}

// CheckHolders returns a list of user names holding a file with fileId
func (s *server) CheckHolders(ctx context.Context, in *pb.CheckHolder) (*pb.ListReply, error) {
    fileId := in.GetFileId()

    holders := fileHolders[fileId]

    holderNames := make([]string, len(holders))
    for i, holder := range holders {
        holderNames[i] = holder.GetName()
    }

    printHoldersMap()

    return &pb.ListReply{Strings: holderNames}, nil
}


// register that the userId holds fileId, then add the user to the list of file holders
func (s *server) RegisterFile(ctx context.Context, in *pb.RegisterRequest) (*emptypb.Empty, error) {
	user := in.GetUser()
	fileId := in.GetFileId()

	// Check if file is held by anyone, don't do anything
	// TODO: perform blockchain transaction here
	if _, ok := fileHolders[fileId]; ok {
		return &emptypb.Empty{}, nil
	}

	fileHolders[fileId] = append(fileHolders[fileId], user)

	return &emptypb.Empty{}, nil
}