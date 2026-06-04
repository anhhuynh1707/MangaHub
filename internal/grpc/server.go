package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"mangahub/internal/auth"
	pb "mangahub/internal/grpc/pb"
	mangaPkg "mangahub/internal/manga"
	userPkg "mangahub/internal/user"
	"mangahub/pkg/models"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MangaServer implements the gRPC MangaService defined in proto/manga.proto.
// It delegates to the existing manga and user services to avoid duplicating business logic.
type MangaServer struct {
	pb.UnimplementedMangaServiceServer
	mangaService *mangaPkg.Service
	userService  *userPkg.Service
	EventHub     *MangaEventHub
}

// NewMangaServer creates a new gRPC MangaServer.
func NewMangaServer(mangaService *mangaPkg.Service, userService *userPkg.Service) *MangaServer {
	return &MangaServer{
		mangaService: mangaService,
		userService:  userService,
		EventHub:     NewMangaEventHub(),
	}
}

// GetManga retrieves a single manga by its slug ID.
func (s *MangaServer) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	if req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "manga_id is required")
	}

	manga, err := s.mangaService.GetByID(req.MangaId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "manga not found: %v", err)
	}

	return mangaToProto(manga), nil
}

// SearchManga queries the manga database by title and/or genre.
func (s *MangaServer) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := &models.MangaSearchQuery{
		Search: req.Query,
		Genre:  req.Genre,
		Page:   1,
		Limit:  limit,
	}

	mangaList, total, err := s.mangaService.Search(query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	var results []*pb.MangaResponse
	for i := range mangaList {
		results = append(results, mangaToProto(&mangaList[i]))
	}

	return &pb.SearchResponse{
		Results: results,
		Total:   int32(total),
	}, nil
}

// UpdateProgress updates a user's reading progress for a manga.
func (s *MangaServer) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "manga_id is required")
	}
	if req.Chapter < 0 {
		return nil, status.Error(codes.InvalidArgument, "chapter must be >= 0")
	}

	progressReq := &models.UpdateProgressRequest{
		MangaID:        req.MangaId,
		CurrentChapter: int(req.Chapter),
		Status:         "reading",
	}

	_, err := s.userService.UpdateProgress(req.UserId, progressReq)
	if err != nil {
		return &pb.ProgressResponse{
			Success: false,
			Message: fmt.Sprintf("failed to update progress: %v", err),
		}, nil
	}

	return &pb.ProgressResponse{
		Success: true,
		Message: fmt.Sprintf("Updated %s to chapter %d", req.MangaId, req.Chapter),
	}, nil
}

// StreamSearch streams search results one manga at a time.
// The client receives each result as a separate message instead of waiting for the full list.
func (s *MangaServer) StreamSearch(req *pb.SearchRequest, stream grpc.ServerStreamingServer[pb.MangaResponse]) error {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := &models.MangaSearchQuery{
		Search: req.Query,
		Genre:  req.Genre,
		Page:   1,
		Limit:  limit,
	}

	mangaList, _, err := s.mangaService.Search(query)
	if err != nil {
		return status.Errorf(codes.Internal, "search failed: %v", err)
	}

	for i := range mangaList {
		if err := stream.Send(mangaToProto(&mangaList[i])); err != nil {
			return err
		}
	}
	return nil
}

// WatchMangaUpdates subscribes the caller to real-time manga events.
// The stream stays open and pushes events (progress updates, manga changes) until the client disconnects.
func (s *MangaServer) WatchMangaUpdates(req *pb.WatchRequest, stream grpc.ServerStreamingServer[pb.MangaEvent]) error {
	ctx := stream.Context()

	subID := fmt.Sprintf("watch-%d", time.Now().UnixNano())
	ch := s.EventHub.Subscribe(subID)
	defer s.EventHub.Unsubscribe(subID)

	// Send an initial connected event so the client knows the stream is live
	if err := stream.Send(&pb.MangaEvent{
		EventType: "connected",
		Message:   fmt.Sprintf("Watching manga updates (filter: %q)", req.MangaId),
		Timestamp: time.Now().Unix(),
	}); err != nil {
		return err
	}

	log.Printf("gRPC Stream: WatchMangaUpdates started for user=%s manga=%q", req.UserId, req.MangaId)

	for {
		select {
		case <-ctx.Done():
			log.Printf("gRPC Stream: client %s disconnected", subID)
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			// Filter: if the request specifies a manga_id, skip unrelated events
			if req.MangaId != "" && event.MangaId != req.MangaId {
				continue
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// mangaToProto converts a models.Manga to the protobuf MangaResponse.
func mangaToProto(m *models.Manga) *pb.MangaResponse {
	return &pb.MangaResponse{
		Id:            m.ID,
		Title:         m.Title,
		Author:        m.Author,
		Genres:        m.Genres,
		Status:        m.Status,
		TotalChapters: int32(m.TotalChapters),
		Description:   m.Description,
	}
}

// AuthInterceptor is a gRPC UnaryServerInterceptor that checks for a valid JWT token.
func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	tokenString := authHeader[0]
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Add claims to context so methods can use the user_id if needed
	// context keys should theoretically be unexported custom types, but string is okay for a quick demo
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "username", claims.Username)

	return handler(ctx, req)
}

// StreamAuthInterceptor validates JWT on server-side streaming RPCs.
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader[0], "Bearer ")
	if _, err := auth.ValidateToken(tokenString); err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	return handler(srv, ss)
}

// ServeGRPC binds a port and starts serving with a pre-created MangaServer.
// Use this when you need the server reference before blocking on Serve.
func ServeGRPC(port string, mangaServer *MangaServer) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("gRPC: failed to listen on :%s: %w", port, err)
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor),
		grpc.StreamInterceptor(StreamAuthInterceptor),
	)
	pb.RegisterMangaServiceServer(srv, mangaServer)

	log.Printf("🔌 gRPC MangaService listening on :%s", port)
	return srv.Serve(lis)
}

// StartGRPCServer is a convenience wrapper for standalone mode.
// It creates the server internally and blocks.
func StartGRPCServer(port string, mangaService *mangaPkg.Service, userService *userPkg.Service) error {
	mangaServer := NewMangaServer(mangaService, userService)
	return ServeGRPC(port, mangaServer)
}
