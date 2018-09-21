package admin

import (
	"context"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"net"
)

var (
	adminPort = flag.Int("adminPort", 9997, "Port of gRPC admin server.")
)

type server struct {
}

func (s *server) AddUser(ctx context.Context, req *AddUserRequest) (*AddUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *server) AddFeed(ctx context.Context, req *AddFeedRequest) (*AddFeedResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func newServer() AdminServiceServer {
	s := &server{}
	return s
}

func Start(ctx context.Context) {
	log.Infof("Starting gRPC admin server.")

	s := grpc.NewServer()

	RegisterAdminServiceServer(s, newServer())
	reflection.Register(s)

	go func(s *grpc.Server) {
		select {
		case <-ctx.Done():
			log.Infof("Shutting down admin server.")
			s.GracefulStop()
		}
	}(s)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *adminPort))
	if err != nil {
		log.Warningf("failed to listen on admin port: %s", err)
		return
	}

	if err := s.Serve(lis); err != nil {
		log.Infof("%s", err)
	}
}
