package client

import (
	"context"
	"fmt"
	"net"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewPersistClientUnix creates a persist client connected via Unix socket (for internal IPC)
func NewPersistClientUnix(socketPath string) (pb.PersistServiceClient, error) {
	// Unix socket dialer - addr parameter is the target address (just the socket path)
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		d.Timeout = 5 * time.Second
		// Dial the Unix socket directly
		return d.DialContext(ctx, "unix", socketPath)
	}

	// Connect to Unix socket without TLS (pure Linux IPC)
	// Use a dummy address since we're overriding the dialer
	// Set large message sizes for streaming historical logs/metrics (128MB each direction)
	conn, err := grpc.NewClient(
		"passthrough:///unix",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(true),
			grpc.MaxCallRecvMsgSize(134217728), // 128MB - handle large historical data streams
			grpc.MaxCallSendMsgSize(134217728), // 128MB - handle large requests
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to persist Unix socket %s: %w", socketPath, err)
	}

	return pb.NewPersistServiceClient(conn), nil
}
