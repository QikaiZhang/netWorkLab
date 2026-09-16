// grpc/client: 调用远端 GetUser，像调用本地函数一样。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "network-lab/grpc/userpb"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.GetUser(ctx, &pb.GetUserRequest{Id: 42})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rpc returned: id=%d name=%s\n", resp.GetId(), resp.GetName())
}
