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
func (s *server) AddUser(_ context.Context, _ *AddUserRequest) (*AddUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// AddFeed adds the specified feed into the database.
// During the operation of adding a feed, fetching is paused and restarted.
func (s *server) AddFeed(_ context.Context, req *AddFeedRequest) (*AddFeedResponse, error) {
	resp := &AddFeedResponse{}
	// Zero indicates the root folder.
	parentID := int64(0)

	if req.Title == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Title")
	}
	if req.URL == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify URL")
	}
	if req.Link == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Link")
	}
	if req.Username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Username")
	}

	feed := models.Feed{
		Title:       req.Title,
		Description: req.Description,
		URL:         req.URL,
		Link:        req.Link,
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "could not find user")
	}

	if req.Folder != "" {
		folders, err := s.db.GetAllFoldersForUser(user)
		if err != nil {
			return nil, status.Error(codes.Internal, "internal error")
		}

		for _, f := range folders {
			if f.Name == req.Folder {
				parentID = f.ID
				break
			}
		}

		if parentID == 0 {
			return nil, status.Error(codes.InvalidArgument, "could not find folder")
		}
	}

	fetch.Pause()
	defer fetch.Resume()

	feedID, err := s.db.InsertFeedForUser(user, feed, parentID)
	if err != nil {
		return resp, status.Error(codes.DataLoss, "failed to persist feed")
	}

	resp.Id = feedID

	// TODO: Indicate if feed already existed.
	return resp, nil
}

// GetFeeds lists all the feeds belonging to the requested user.
func (s *server) GetFeeds(_ context.Context, req *GetFeedsRequest) (*GetFeedsResponse, error) {
	resp := &GetFeedsResponse{}

	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "must specify Username")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Error(codes.NotFound, "could not find user")
	}

	feeds, err := s.db.GetAllFeedsForUser(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	for _, f := range feeds {
		resp.Feeds = append(resp.Feeds, &GetFeedsResponse_Feed{Id: f.ID, Title: f.Title})
	}

	return resp, nil
}

// RemoveFeed removes the requested feed for the requested user.
// During the operation of adding a feed, fetching is paused and restarted.
func (s *server) RemoveFeed(_ context.Context, req *RemoveFeedRequest) (*RemoveFeedResponse, error) {
	resp := &RemoveFeedResponse{}

	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "must specify Username")
	}
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "must specify non-zero Id")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Error(codes.NotFound, "could not find user")
	}

	feeds, err := s.db.GetAllFeedsForUser(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	var folderId int64 = -1
	for _, f := range feeds {
		if f.ID == req.Id {
			folderId = f.FolderID
			break
		}
	}
	if folderId == -1 {
		return nil, status.Error(codes.InvalidArgument, "could not find feed")
	}

	fetch.Pause()
	defer fetch.Resume()

	err = s.db.DeleteFeedForUser(user, req.Id, folderId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

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
