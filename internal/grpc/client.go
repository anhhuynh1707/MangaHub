package grpc

import (
	"context"
	"io"
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

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, err
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

// GetManga fetches a single manga by ID via gRPC.
func (c *MangaClient) GetManga(mangaID string) (*pb.MangaResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = c.withAuth(ctx)

	return c.client.GetManga(ctx, &pb.GetMangaRequest{
		MangaId: mangaID,
	})
}

// SearchManga searches manga by title and/or genre via gRPC.
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

// UpdateProgress updates reading progress via gRPC.
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

// StreamSearch streams manga results one by one via server-side streaming.
// onResult is called for each received MangaResponse until the stream ends.
func (c *MangaClient) StreamSearch(query, genre string, limit int32, onResult func(*pb.MangaResponse)) error {
	ctx := c.withAuth(context.Background())
	stream, err := c.client.StreamSearch(ctx, &pb.SearchRequest{
		Query: query,
		Genre: genre,
		Limit: limit,
	})
	if err != nil {
		return err
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		onResult(resp)
	}
}

// WatchMangaUpdates subscribes to real-time manga events via server-side streaming.
// onEvent is called for each event. The stream runs until ctx is cancelled or the server closes it.
func (c *MangaClient) WatchMangaUpdates(ctx context.Context, mangaID, userID string, onEvent func(*pb.MangaEvent)) error {
	ctx = c.withAuth(ctx)
	stream, err := c.client.WatchMangaUpdates(ctx, &pb.WatchRequest{
		MangaId: mangaID,
		UserId:  userID,
	})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		onEvent(event)
	}
}
