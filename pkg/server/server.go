package server

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	pb "github.com/dovics/keda-ingress-nginx-scaler/pkg/api"
)

type Server struct {
	port   int
	server *grpc.Server
}

func NewServer(port int) *Server {
	grpcServer := grpc.NewServer()

	return &Server{
		port:   port,
		server: grpcServer,
	}
}

func (s *Server) Start(srv pb.ExternalScalerServer) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", s.port))
	if err != nil {
		return err
	}
	pb.RegisterExternalScalerServer(s.server, srv)

	klog.V(2).Infof("listenting on %d", s.port)
	if err := s.server.Serve(lis); err != nil {
		log.Fatal(err)
	}

	return nil
}
