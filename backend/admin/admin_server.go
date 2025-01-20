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
	"sort"
	"strings"
)

var (
	adminPort = flag.Int("adminPort", 9997, "Port of gRPC admin server.")
)

type server struct {
	UnimplementedAdminServiceServer
	db storage.Database
}

// AddUser adds a specified user into the database.
// NOTE: This method is currently unimplemented.
func (s *server) AddUser(_ context.Context, _ *AddUserRequest) (*AddUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// GetMuteWords retrieves the current muted words for the user.
func (s *server) GetMuteWords(_ context.Context, req *GetMuteWordsRequest) (*GetMuteWordsResponse, error) {
	resp := &GetMuteWordsResponse{}

	if req.Username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Username")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "could not find user")
	}

	mutedWords, err := s.db.GetMuteWordsForUser(user)
	if err != nil {
		log.Warningf("while retrieving muted words for user: %+v", err)
		return nil, status.Errorf(codes.Internal, "could not retrieve muted words for user")
	}

	for _, m := range mutedWords {
		resp.MuteWord = append(resp.MuteWord, m)
	}

	return resp, nil
}

// AddMuteWord adds specified mute words for a user.
func (s *server) AddMuteWord(_ context.Context, req *AddMuteWordRequest) (*AddMuteWordResponse, error) {
	resp := &AddMuteWordResponse{}

	if req.Username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Username")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "could not find user")
	}

	if len(req.GetMuteWord()) == 0 {
		return resp, nil
	}

	// Make a unique, sorted list from the input
	uniqueMap := make(map[string]bool)
	for _, str := range req.GetMuteWord() {
		uniqueMap[strings.ToLower(str)] = true
	}
	muteWords := make([]string, 0, len(uniqueMap))
	for str := range uniqueMap {
		muteWords = append(muteWords, strings.ToLower(str))
	}
	sort.Strings(muteWords)

	err = s.db.UpdateMuteWordsForUser(user, muteWords)
	if err != nil {
		log.Warningf("while adding muted words for user: %+v", err)
		return nil, status.Errorf(codes.Internal, "could not insert mute words")
	}

	return resp, nil
}

// DeleteMuteWord removes the specified mute words for a user.
func (s *server) DeleteMuteWord(_ context.Context, req *DeleteMuteWordRequest) (*DeleteMuteWordResponse, error) {
	resp := &DeleteMuteWordResponse{}

	if req.Username == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify Username")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "could not find user")
	}

	if len(req.GetMuteWord()) == 0 {
		return resp, nil
	}

	// Make a unique, sorted list from the input
	uniqueMap := make(map[string]bool)
	for _, str := range req.GetMuteWord() {
		uniqueMap[strings.ToLower(str)] = true
	}
	muteWords := make([]string, 0, len(uniqueMap))
	for str := range uniqueMap {
		muteWords = append(muteWords, strings.ToLower(str))
	}
	sort.Strings(muteWords)

	err = s.db.DeleteMuteWordsForUser(user, muteWords)
	if err != nil {
		log.Warningf("while deleting muted words for user: %+v", err)
		return nil, status.Errorf(codes.Internal, "could not delete mute words")
	}

	return resp, nil
}

// AddFeed adds the specified feed into the database.
// During the operation of adding a feed, fetching is paused and restarted.
func (s *server) AddFeed(_ context.Context, req *AddFeedRequest) (*AddFeedResponse, error) {
	resp := &AddFeedResponse{}
	folderID := int64(-1)

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
				folderID = f.ID
				break
			}
		}
	}

	fetch.Pause()
	defer fetch.Resume()

	// Folder ID not found, so create a new one
	if folderID == -1 {
		newFolder := models.Folder{Name: req.Folder}
		folderID, err = s.db.InsertFolderForUser(user, newFolder, 0)
		if err != nil {
			return nil, status.Error(codes.DataLoss, "could not create new folder")
		}
	}

	feedID, err := s.db.InsertFeedForUser(user, feed, folderID)
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

// EditFeed updates the requested feed for the requested user.
// During the operation of adding a feed, fetching is paused and restarted.
func (s *server) EditFeed(_ context.Context, req *EditFeedRequest) (*EditFeedResponse, error) {
	resp := &EditFeedResponse{}

	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "must specify Username")
	}
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "must specify non-zero Feed ID")
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, status.Error(codes.NotFound, "could not find user")
	}

	folders, err := s.db.GetAllFoldersForUser(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	var folderId int64 = -1
	for _, f := range folders {
		if f.Name == req.Folder {
			folderId = f.ID
			break
		}
	}
	if folderId == -1 {
		return nil, status.Error(codes.InvalidArgument, "could not find folder")
	}

	fetch.Pause()
	defer fetch.Resume()

	err = s.db.UpdateFolderForFeedForUser(user, req.Id, folderId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	return resp, nil
}

func newServer(d storage.Database) AdminServiceServer {
	s := &server{db: d}
	return s
}

// Start starts the gRPC admin server.
func Start(ctx context.Context, d storage.Database) {
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
