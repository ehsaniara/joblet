package ipc

import (
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// readIPCMessage reads one length-prefixed IPC message from the connection
func readIPCMessage(t *testing.T, conn net.Conn) *ipcpb.IPCMessage {
	t.Helper()

	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBuf); err != nil {
		t.Fatalf("failed to read length prefix: %v", err)
	}
	data := make([]byte, binary.BigEndian.Uint32(lengthBuf))
	if _, err := io.ReadFull(conn, data); err != nil {
		t.Fatalf("failed to read message body: %v", err)
	}

	msg := &ipcpb.IPCMessage{}
	if err := proto.Unmarshal(data, msg); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	return msg
}

// TestWriterBuffersWhileDisconnected verifies that messages written before
// persist is listening are queued and delivered once the connection is up,
// instead of being dropped. This is the service-warmup window: the first job
// after a restart produces logs before the ipc-writer has connected.
func TestWriterBuffersWhileDisconnected(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "persist-test.sock")

	w := NewWriter(&Config{
		Socket:         socket,
		BufferSize:     16,
		ReconnectDelay: 50 * time.Millisecond,
	}, logger.New())
	defer w.Close()

	// Persist is NOT up yet - this write must be accepted and buffered
	if err := w.WriteLog("warmup-job", ipcpb.StreamType_STREAM_TYPE_STDOUT, 1, 1, []byte("early line")); err != nil {
		t.Fatalf("write while disconnected should buffer, got error: %v", err)
	}

	// Bring persist up after the write
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	defer listener.Close()

	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			connCh <- conn
		}
	}()

	var conn net.Conn
	select {
	case conn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not connect to persist within 5s")
	}
	defer conn.Close()

	// The buffered message must arrive
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}
	msg := readIPCMessage(t, conn)

	if msg.JobUuid != "warmup-job" {
		t.Errorf("expected buffered message for warmup-job, got %q", msg.JobUuid)
	}
	if msg.Type != ipcpb.MessageType_MESSAGE_TYPE_LOG {
		t.Errorf("expected log message, got %v", msg.Type)
	}

	logLine := &ipcpb.LogLine{}
	if err := proto.Unmarshal(msg.Data, logLine); err != nil {
		t.Fatalf("failed to unmarshal log line: %v", err)
	}
	if string(logLine.Content) != "early line" {
		t.Errorf("expected buffered content %q, got %q", "early line", string(logLine.Content))
	}
}
