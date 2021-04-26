package main

import (
	"log"
	"net"

	grpcManager "github.com/loisBN/zippytal-desktop/back/grpc_manager"
	"google.golang.org/grpc"
)

func main() {
	lis,err := net.Listen("tcp",":8080")
	if err != nil {
		log.Fatalln(err)
	}
	grpcServer := grpc.NewServer(grpc.MaxConcurrentStreams(100000))
	grpcManager.RegisterGrpcManagerServer(grpcServer,grpcManager.NewGRPCManagerService())
	log.Fatalln(grpcServer.Serve(lis))
}