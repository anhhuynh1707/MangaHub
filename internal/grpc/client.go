package grpc

import (
	"context"
	"fmt"
	"time"

	pb "mangahub/internal/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// MangaClient wraps the gRPC MangaService client with connection management.
type MangaClient struct {
	conn   *grpc.ClientConn
	client pb.MangaServiceClient
	token  string
}

// NewMangaClient connects to the gRPC server at the given address.
func NewMangaClient(addr, token string) (*MangaClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("gRPC: failed to connect to %s: %w", addr, err)
	}

	return &MangaClient{
		conn:   conn,
		client: pb.NewMangaServiceClient(conn),
		token:  token,
	}, nil
}

func (c *MangaClient) withAuth(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
}

// Close closes the underlying gRPC connection.
func (c *MangaClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetManga retrieves a single manga by its slug ID.
func (c *MangaClient) GetManga(mangaID string) (*pb.MangaResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = c.withAuth(ctx)

	return c.client.GetManga(ctx, &pb.GetMangaRequest{
		MangaId: mangaID,
	})
}

// SearchManga queries the manga database by title and/or genre.
func (c *MangaClient) SearchManga(query, genre string, limit int32) (*pb.SearchResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = c.withAuth(ctx)

	return c.client.SearchManga(ctx, &pb.SearchRequest{
		Query: query,
		Genre: genre,
		Limit: limit,
	})
}

// UpdateProgress updates a user's reading progress for a manga.
func (c *MangaClient) UpdateProgress(userID, mangaID string, chapter int32) (*pb.ProgressResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = c.withAuth(ctx)

	return c.client.UpdateProgress(ctx, &pb.ProgressRequest{
		UserId:  userID,
		MangaId: mangaID,
		Chapter: chapter,
	})
}
