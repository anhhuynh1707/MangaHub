package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

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
}

// NewMangaServer creates a new gRPC MangaServer.
func NewMangaServer(mangaService *mangaPkg.Service, userService *userPkg.Service) *MangaServer {
	return &MangaServer{
		mangaService: mangaService,
		userService:  userService,
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

// StartGRPCServer starts the gRPC server on the given port.
// It blocks until the server is stopped or an error occurs.
func StartGRPCServer(port string, mangaService *mangaPkg.Service, userService *userPkg.Service) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("gRPC: failed to listen on :%s: %w", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor),
	)
	mangaServer := NewMangaServer(mangaService, userService)
	pb.RegisterMangaServiceServer(grpcServer, mangaServer)

	log.Printf("🔌 gRPC MangaService listening on :%s", port)
	return grpcServer.Serve(lis)
}
