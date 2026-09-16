// grpc/server: 在 :50051 上提供最小 gRPC 服务（HTTP/2 明文 h2c，实验用）。
package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "network-lab/grpc/userpb"
)

type server struct {
	pb.UnimplementedUserServiceServer
}

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	fmt.Printf("rpc GetUser(id=%d)\n", req.GetId())
	return &pb.GetUserResponse{Id: req.GetId(), Name: fmt.Sprintf("user-%d", req.GetId())}, nil
}

func main() {
	ln, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &server{})
	fmt.Println("grpc server listening on :50051")
	log.Fatal(s.Serve(ln))
}
