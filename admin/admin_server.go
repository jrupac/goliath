package admin

import (
	"context"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
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
	db *storage.Database
}

// AddUser adds a specified user into the database.
// NOTE: This method is currently unimplemented.
func (s *server) AddUser(ctx context.Context, req *AddUserRequest) (*AddUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// AddFeed adds the specified feed into the database.
// During the operation of adding a feed, fetching is paused and restarted.
func (s *server) AddFeed(ctx context.Context, req *AddFeedRequest) (*AddFeedResponse, error) {
	resp := &AddFeedResponse{}
	// Zero indicates the root folder.
	parentID := int64(0)

	feed := models.Feed{
		Title:       req.Title,
		Description: req.Description,
		URL:         req.URL,
		Link:        req.Link,
	}

	if req.Folder != "" {
		folders, err := s.db.GetAllFolders()
		if err != nil {
			return nil, status.Error(codes.Internal, "internal error")
		}

		// TODO: Handle case where folder is not found.
		for _, f := range folders {
			if f.Name == req.Folder {
				parentID = f.ID
				break
			}
		}
	}

	fetch.Pause()
	defer fetch.Resume()
	feedID, err := s.db.InsertFeed(feed, parentID)

	if err != nil {
		return resp, status.Error(codes.DataLoss, "failed to persist feed")
	}
	resp.Id = feedID

	// TODO: Indicate if feed already existed.
	return resp, nil
}

func newServer(d *storage.Database) AdminServiceServer {
	s := &server{db: d}
	return s
}

// Start starts the gRPC admin server.
func Start(ctx context.Context, d *storage.Database) {
	log.Infof("Starting gRPC admin server.")

	s := grpc.NewServer()

	RegisterAdminServiceServer(s, newServer(d))
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
