package nvrsnapshot

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"sync"
	"testing"
	"time"
)

func TestWebSocketJPEGCaptureFeedsParameterSetsAndFUKeyFrameToFFmpeg(t *testing.T) {
	output := testJPEG(t, 640, 360)
	conn := &fakeWebSocketConn{messages: []WebSocketMessage{
		binaryMessage(rtpTimestampPacket(1, 100, true, h265NAL(32, 0x32))),
		binaryMessage(rtpTimestampPacket(2, 100, true, h265NAL(33, 0x33))),
		binaryMessage(rtpTimestampPacket(3, 100, true, h265NAL(34, 0x34))),
		binaryMessage(rtpTimestampPacket(4, 200, false, []byte{0x62, 0x01, 0x80 | 19, 0xaa})),
		binaryMessage(rtpTimestampPacket(5, 200, true, []byte{0x62, 0x01, 0x40 | 19, 0xbb})),
	}}
	command := &fakeFFmpegCommand{output: output}
	factory := &fakeCommandFactory{command: command}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, factory)

	got, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream")
	if code != "" {
		t.Fatalf("Capture() code = %q", code)
	}
	if got.ContentType != "image/jpeg" || got.Width != 640 || got.Height != 360 || !bytes.Equal(got.Bytes, output) {
		t.Fatalf("Capture() = %+v, want JPEG metadata and bytes", got)
	}

	wantMedia := append([]byte{}, annexB(h265NAL(32, 0x32))...)
	wantMedia = append(wantMedia, annexB(h265NAL(33, 0x33))...)
	wantMedia = append(wantMedia, annexB(h265NAL(34, 0x34))...)
	wantMedia = append(wantMedia, []byte{0, 0, 0, 1, 0x26, 0x01, 0xaa, 0xbb}...)
	if !bytes.Equal(command.input.Bytes(), wantMedia) {
		t.Fatalf("ffmpeg stdin = %x, want %x", command.input.Bytes(), wantMedia)
	}
	wantArgs := []string{
		"-hide_banner", "-loglevel", "error", "-f", "hevc", "-i", "pipe:0",
		"-frames:v", "1", "-vf", "scale=640:640:force_original_aspect_ratio=decrease", "-q:v", "5", "-f", "image2", "pipe:1",
	}
	if !equalStrings(factory.args, wantArgs) {
		t.Fatalf("ffmpeg args = %#v, want %#v", factory.args, wantArgs)
	}
	if !conn.closed || !command.stdin.closed || !command.stdout.closed || !command.waited {
		t.Fatalf("resources not closed: websocket=%t stdin=%t stdout=%t waited=%t", conn.closed, command.stdin.closed, command.stdout.closed, command.waited)
	}
}

func TestWebSocketJPEGCaptureWaitsForMarkerAndIncludesAllKeyFrameSlices(t *testing.T) {
	output := testJPEG(t, 640, 360)
	conn := &fakeWebSocketConn{messages: []WebSocketMessage{
		binaryMessage(rtpTimestampPacket(1, 100, true, h265NAL(32, 0x32))),
		binaryMessage(rtpTimestampPacket(2, 100, true, h265NAL(33, 0x33))),
		binaryMessage(rtpTimestampPacket(3, 100, true, h265NAL(34, 0x34))),
		binaryMessage(rtpTimestampPacket(4, 200, false, h265NAL(19, 0xa1))),
		binaryMessage(rtpTimestampPacket(5, 200, true, h265NAL(19, 0xa2))),
	}}
	command := &fakeFFmpegCommand{output: output}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})

	_, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream")
	if code != "" {
		t.Fatalf("Capture() code = %q", code)
	}
	want := append([]byte{}, annexB(h265NAL(32, 0x32))...)
	want = append(want, annexB(h265NAL(33, 0x33))...)
	want = append(want, annexB(h265NAL(34, 0x34))...)
	want = append(want, annexB(h265NAL(19, 0xa1))...)
	want = append(want, annexB(h265NAL(19, 0xa2))...)
	if !bytes.Equal(command.input.Bytes(), want) {
		t.Fatalf("ffmpeg stdin = %x, want complete key access unit %x", command.input.Bytes(), want)
	}
}

func TestWebSocketJPEGCaptureWaitsForAllParameterSetsBeforeKeyAccessUnit(t *testing.T) {
	output := testJPEG(t, 640, 360)
	conn := &fakeWebSocketConn{messages: []WebSocketMessage{
		binaryMessage(rtpTimestampPacket(1, 100, true, h265NAL(32, 0x32))),
		binaryMessage(rtpTimestampPacket(2, 100, true, h265NAL(33, 0x33))),
		binaryMessage(rtpTimestampPacket(3, 200, true, h265NAL(19, 0xa1))),
		binaryMessage(rtpTimestampPacket(4, 300, true, h265NAL(34, 0x34))),
		binaryMessage(rtpTimestampPacket(5, 400, true, h265NAL(19, 0xa2))),
	}}
	command := &fakeFFmpegCommand{output: output}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})

	if _, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream"); code != "" {
		t.Fatalf("Capture() code = %q", code)
	}
	want := append([]byte{}, annexB(h265NAL(32, 0x32))...)
	want = append(want, annexB(h265NAL(33, 0x33))...)
	want = append(want, annexB(h265NAL(34, 0x34))...)
	want = append(want, annexB(h265NAL(19, 0xa2))...)
	if !bytes.Equal(command.input.Bytes(), want) {
		t.Fatalf("ffmpeg stdin = %x, want later key access unit %x", command.input.Bytes(), want)
	}
}

func TestWebSocketJPEGCaptureIgnoresParameterSetMarkerForOpenKeyAccessUnit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	messages := parameterSetMessages()
	messages = append(messages,
		binaryMessage(rtpTimestampPacket(4, 200, false, h265NAL(19, 0xa1))),
		binaryMessage(rtpTimestampPacket(5, 300, true, h265NAL(34, 0x35))),
	)
	command := &fakeFFmpegCommand{output: testJPEG(t, 640, 360)}
	conn := &fakeWebSocketConn{messages: messages}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})

	if _, code := capture.Capture(ctx, "wss://temporary.example.invalid/stream"); code != ErrorMediaTimeout {
		t.Fatalf("Capture() code = %q, want %q", code, ErrorMediaTimeout)
	}
	if command.started {
		t.Fatal("parameter-set marker incorrectly started ffmpeg")
	}
}

func TestCaptureServiceWhitelistMapsUnknownDependencyCodes(t *testing.T) {
	service := NewCaptureService(
		fakeAuthorizer{code: ErrorCode("token=secret")},
		fakeJPEGCapture{code: ErrorCode("wss://secret.example.invalid")},
	)
	if _, code := service.Capture(context.Background(), 7, StreamRequest{Mode: StreamModeLive}); code != ErrorAuthorizationFailed {
		t.Fatalf("unknown authorizer code = %q, want %q", code, ErrorAuthorizationFailed)
	}

	service = NewCaptureService(
		fakeAuthorizer{url: "wss://temporary.example.invalid/stream"},
		fakeJPEGCapture{code: ErrorCode("wss://secret.example.invalid")},
	)
	if _, code := service.Capture(context.Background(), 7, StreamRequest{Mode: StreamModeLive}); code != ErrorDecodeFailed {
		t.Fatalf("unknown capture code = %q, want %q", code, ErrorDecodeFailed)
	}
}

func TestWebSocketJPEGCaptureRejectsInvalidRTPVersionAndPayloadType(t *testing.T) {
	tests := []struct {
		name   string
		packet []byte
	}{
		{name: "version", packet: append([]byte{0x40, 96}, make([]byte, 10)...)},
		{name: "payload type", packet: append([]byte{0x80, 97}, make([]byte, 10)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeWebSocketConn{messages: []WebSocketMessage{binaryMessage(tt.packet)}}
			capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{})

			_, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream")
			if code != ErrorDemuxFailed {
				t.Fatalf("Capture() code = %q, want %q", code, ErrorDemuxFailed)
			}
			if !conn.closed {
				t.Fatal("websocket was not closed")
			}
		})
	}
}

func TestWebSocketJPEGCaptureMapsNoCompleteMediaAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	conn := &fakeWebSocketConn{messages: []WebSocketMessage{
		binaryMessage(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})),
	}}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{})

	_, code := capture.Capture(ctx, "wss://temporary.example.invalid/stream")
	if code != ErrorMediaTimeout {
		t.Fatalf("Capture() code = %q, want %q", code, ErrorMediaTimeout)
	}
	if !conn.closed {
		t.Fatal("websocket was not closed")
	}
}

func TestWebSocketJPEGCaptureMapsWebSocketFailures(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want ErrorCode
	}{
		{name: "failure", ctx: context.Background(), err: errors.New("dial failed"), want: ErrorWSSConnectFailed},
		{name: "deadline", ctx: expiredContext(), err: context.DeadlineExceeded, want: ErrorWSSConnectTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{err: tt.err}, &fakeCommandFactory{})
			_, code := capture.Capture(tt.ctx, "wss://temporary.example.invalid/stream")
			if code != tt.want {
				t.Fatalf("Capture() code = %q, want %q", code, tt.want)
			}
		})
	}
}

func TestWebSocketJPEGCaptureRejectsNonBinaryAndMalformedRTP(t *testing.T) {
	tests := []struct {
		name    string
		message WebSocketMessage
	}{
		{name: "text", message: WebSocketMessage{Type: WebSocketMessageText, Data: []byte("not RTP")}},
		{name: "short RTP", message: binaryMessage([]byte{0x80, 96})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: &fakeWebSocketConn{messages: []WebSocketMessage{tt.message}}}, &fakeCommandFactory{})
			_, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream")
			if code != ErrorDemuxFailed {
				t.Fatalf("Capture() code = %q, want %q", code, ErrorDemuxFailed)
			}
		})
	}
}

func TestWebSocketJPEGCaptureValidatesJPEGOutput(t *testing.T) {
	valid := testJPEG(t, 640, 360)
	tests := []struct {
		name string
		jpeg []byte
		want ErrorCode
	}{
		{name: "undecodable", jpeg: []byte("not a JPEG"), want: ErrorDecodeFailed},
		{name: "too wide", jpeg: testJPEG(t, 641, 360), want: ErrorThumbnailInvalid},
		{name: "too large", jpeg: append(valid, make([]byte, maxJPEGBytes-len(valid)+1)...), want: ErrorThumbnailInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeWebSocketConn{messages: keyFrameMessages()}
			command := &fakeFFmpegCommand{output: tt.jpeg}
			capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})

			_, code := capture.Capture(context.Background(), "wss://temporary.example.invalid/stream")
			if code != tt.want {
				t.Fatalf("Capture() code = %q, want %q", code, tt.want)
			}
			if !conn.closed || !command.stdin.closed || !command.stdout.closed || !command.waited {
				t.Fatalf("resources not closed: websocket=%t stdin=%t stdout=%t waited=%t", conn.closed, command.stdin.closed, command.stdout.closed, command.waited)
			}
		})
	}
}

func TestValidateJPEGClearsRejectedImageBytes(t *testing.T) {
	data := append(make([]byte, 0, 32), []byte("not a JPEG")...)
	if _, code := validateJPEG(data); code != ErrorDecodeFailed {
		t.Fatalf("validateJPEG() code = %q, want %q", code, ErrorDecodeFailed)
	}
	for index, value := range data[:cap(data)] {
		if value != 0 {
			t.Fatalf("rejected JPEG byte %d = %d, want 0", index, value)
		}
	}
}

func TestWebSocketJPEGCaptureReadsStdoutConcurrentlyWithStdin(t *testing.T) {
	readStarted := make(chan struct{})
	command := &fakeFFmpegCommand{output: testJPEG(t, 640, 360)}
	command.stdin.waitForRead = readStarted
	command.stdout.onRead = func() { close(readStarted) }
	conn := &fakeWebSocketConn{messages: keyFrameMessages()}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, code := capture.Capture(ctx, "wss://temporary.example.invalid/stream"); code != "" {
		t.Fatalf("Capture() code = %q", code)
	}
	assertCommandResourcesClosed(t, conn, command, true, false)
}

func TestWebSocketJPEGCaptureCleansFFmpegResourcesOnEveryFailure(t *testing.T) {
	valid := testJPEG(t, 640, 360)
	tests := []struct {
		name        string
		configure   func(*fakeFFmpegCommand)
		context     func() (context.Context, context.CancelFunc)
		want        ErrorCode
		wantStarted bool
		wantKilled  bool
	}{
		{
			name:      "start",
			configure: func(command *fakeFFmpegCommand) { command.startErr = errors.New("start") },
			context:   func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:      ErrorDecodeFailed,
		},
		{
			name:      "stdin write",
			configure: func(command *fakeFFmpegCommand) { command.stdin.writeErr = errors.New("write") },
			context:   func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:      ErrorDecodeFailed, wantStarted: true, wantKilled: true,
		},
		{
			name:      "stdin close",
			configure: func(command *fakeFFmpegCommand) { command.stdin.closeErr = errors.New("close") },
			context:   func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:      ErrorDecodeFailed, wantStarted: true, wantKilled: true,
		},
		{
			name:      "stdout read",
			configure: func(command *fakeFFmpegCommand) { command.stdout.readErr = errors.New("read") },
			context:   func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:      ErrorDecodeFailed, wantStarted: true, wantKilled: true,
		},
		{
			name: "stdout too large",
			configure: func(command *fakeFFmpegCommand) {
				command.output = append(valid, make([]byte, maxJPEGBytes-len(valid)+1)...)
			},
			context: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:    ErrorThumbnailInvalid, wantStarted: true, wantKilled: true,
		},
		{
			name:      "wait",
			configure: func(command *fakeFFmpegCommand) { command.waitErr = errors.New("wait") },
			context:   func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			want:      ErrorDecodeFailed, wantStarted: true,
		},
		{
			name:      "context cancellation",
			configure: func(command *fakeFFmpegCommand) { command.stdout.blockUntilClose = true },
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 20*time.Millisecond)
			},
			want: ErrorMediaTimeout, wantStarted: true, wantKilled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.context()
			defer cancel()
			command := &fakeFFmpegCommand{output: valid}
			tt.configure(command)
			conn := &fakeWebSocketConn{messages: keyFrameMessages()}
			capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})

			if _, code := capture.Capture(ctx, "wss://temporary.example.invalid/stream"); code != tt.want {
				t.Fatalf("Capture() code = %q, want %q", code, tt.want)
			}
			assertCommandResourcesClosed(t, conn, command, tt.wantStarted, tt.wantKilled)
		})
	}
}

func TestWebSocketJPEGCaptureWaitsForBlockedStdinWriterOnCancellation(t *testing.T) {
	neverRead := make(chan struct{})
	command := &fakeFFmpegCommand{output: testJPEG(t, 640, 360)}
	command.stdin.waitForRead = neverRead
	command.stdout.blockUntilClose = true
	conn := &fakeWebSocketConn{messages: keyFrameMessages()}
	capture := NewWebSocketJPEGCapture(&fakeWebSocketDialer{conn: conn}, &fakeCommandFactory{command: command})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, code := capture.Capture(ctx, "wss://temporary.example.invalid/stream"); code != ErrorMediaTimeout {
		t.Fatalf("Capture() code = %q, want %q", code, ErrorMediaTimeout)
	}
	assertCommandResourcesClosed(t, conn, command, true, true)
	if !command.stdin.writeFinished {
		t.Fatal("Capture() returned before blocked stdin writer finished")
	}
	if command.input.Len() != 0 {
		t.Fatalf("stdin wrote %d bytes after cancellation", command.input.Len())
	}
}

func TestParameterSetReplacementClearsReusedBackingStorage(t *testing.T) {
	first := append(make([]byte, 0, 16), []byte{1, 2, 3, 4, 5, 6}...)
	updated := replaceParameterSet(first, []byte{9, 8})
	for index, value := range updated[2:cap(updated)] {
		if value != 0 {
			t.Fatalf("reused parameter-set tail byte %d = %d, want 0", index+2, value)
		}
	}

	parameterSets := [3][]byte{updated}
	clearParameterSets(&parameterSets)
	for index, value := range updated[:cap(updated)] {
		if value != 0 {
			t.Fatalf("cleared parameter-set byte %d = %d, want 0", index, value)
		}
	}
}

func keyFrameMessages() []WebSocketMessage {
	messages := parameterSetMessages()
	return append(messages, binaryMessage(rtpTimestampPacket(4, 200, true, h265NAL(19, 0x19))))
}

func parameterSetMessages() []WebSocketMessage {
	return []WebSocketMessage{
		binaryMessage(rtpTimestampPacket(1, 100, true, h265NAL(32, 0x32))),
		binaryMessage(rtpTimestampPacket(2, 100, true, h265NAL(33, 0x33))),
		binaryMessage(rtpTimestampPacket(3, 100, true, h265NAL(34, 0x34))),
	}
}

func assertCommandResourcesClosed(t *testing.T, conn *fakeWebSocketConn, command *fakeFFmpegCommand, wantStarted, wantKilled bool) {
	t.Helper()
	if !conn.closed || !command.stdin.closed || !command.stdout.closed {
		t.Fatalf("resources not closed: websocket=%t stdin=%t stdout=%t", conn.closed, command.stdin.closed, command.stdout.closed)
	}
	if command.started != wantStarted || command.killed != wantKilled {
		t.Fatalf("process state: started=%t killed=%t, want %t %t", command.started, command.killed, wantStarted, wantKilled)
	}
	if wantStarted && !command.waited {
		t.Fatal("started process was not waited")
	}
}

func h265NAL(nalType byte, value byte) []byte {
	return []byte{nalType << 1, 0x01, value}
}

func binaryMessage(data []byte) WebSocketMessage {
	return WebSocketMessage{Type: WebSocketMessageBinary, Data: data}
}

func testJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	imageData.Set(0, 0, color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff})
	var output bytes.Buffer
	if err := jpeg.Encode(&output, imageData, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return output.Bytes()
}

func expiredContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

type fakeWebSocketDialer struct {
	conn *fakeWebSocketConn
	err  error
}

func (d *fakeWebSocketDialer) Dial(context.Context, string) (WebSocketConn, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.conn, nil
}

type fakeAuthorizer struct {
	url  string
	code ErrorCode
}

func (a fakeAuthorizer) AuthorizeStream(context.Context, int64, StreamRequest) (string, ErrorCode) {
	return a.url, a.code
}

type fakeJPEGCapture struct {
	code ErrorCode
}

func (c fakeJPEGCapture) Capture(context.Context, string) (JPEG, ErrorCode) {
	return JPEG{}, c.code
}

type fakeWebSocketConn struct {
	messages []WebSocketMessage
	index    int
	closed   bool
}

func (c *fakeWebSocketConn) Read(ctx context.Context) (WebSocketMessage, error) {
	if c.index < len(c.messages) {
		message := c.messages[c.index]
		c.index++
		return message, nil
	}
	<-ctx.Done()
	return WebSocketMessage{}, ctx.Err()
}

func (c *fakeWebSocketConn) Close() error {
	c.closed = true
	return nil
}

type fakeCommandFactory struct {
	command *fakeFFmpegCommand
	args    []string
}

func (f *fakeCommandFactory) New(_ context.Context, args ...string) (FFmpegCommand, error) {
	f.args = append([]string(nil), args...)
	if f.command == nil {
		f.command = &fakeFFmpegCommand{}
	}
	f.command.initialize()
	return f.command, nil
}

type fakeFFmpegCommand struct {
	input    bytes.Buffer
	output   []byte
	stdin    fakeWriteCloser
	stdout   fakeReadCloser
	startErr error
	waitErr  error
	started  bool
	waited   bool
	killed   bool
}

func (c *fakeFFmpegCommand) initialize() {
	c.stdin.initialize()
	c.stdout.initialize()
}

func (c *fakeFFmpegCommand) StdinPipe() (io.WriteCloser, error) {
	c.stdin.Writer = &c.input
	return &c.stdin, nil
}

func (c *fakeFFmpegCommand) StdoutPipe() (io.ReadCloser, error) {
	c.stdout.Reader = bytes.NewReader(c.output)
	return &c.stdout, nil
}

func (c *fakeFFmpegCommand) Start() error {
	if c.startErr != nil {
		return c.startErr
	}
	c.started = true
	return nil
}

func (c *fakeFFmpegCommand) Wait() error {
	c.waited = true
	return c.waitErr
}

func (c *fakeFFmpegCommand) Kill() error {
	c.killed = true
	return nil
}

type fakeWriteCloser struct {
	io.Writer
	closed        bool
	writeFinished bool
	writeErr      error
	closeErr      error
	waitForRead   <-chan struct{}
	closedCh      chan struct{}
	closeOnce     sync.Once
}

func (c *fakeWriteCloser) initialize() {
	if c.closedCh == nil {
		c.closedCh = make(chan struct{})
	}
}

func (c *fakeWriteCloser) Write(data []byte) (int, error) {
	defer func() { c.writeFinished = true }()
	if c.waitForRead != nil {
		select {
		case <-c.waitForRead:
		case <-c.closedCh:
			return 0, io.ErrClosedPipe
		}
	}
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.Writer.Write(data)
}

func (c *fakeWriteCloser) Close() error {
	c.closeOnce.Do(func() {
		c.closed = true
		close(c.closedCh)
	})
	return c.closeErr
}

type fakeReadCloser struct {
	io.Reader
	closed          bool
	readErr         error
	onRead          func()
	readOnce        sync.Once
	blockUntilClose bool
	closedCh        chan struct{}
	closeOnce       sync.Once
}

func (c *fakeReadCloser) initialize() {
	if c.closedCh == nil {
		c.closedCh = make(chan struct{})
	}
}

func (c *fakeReadCloser) Read(data []byte) (int, error) {
	c.readOnce.Do(func() {
		if c.onRead != nil {
			c.onRead()
		}
	})
	if c.blockUntilClose {
		<-c.closedCh
		return 0, io.EOF
	}
	if c.readErr != nil {
		return 0, c.readErr
	}
	return c.Reader.Read(data)
}

func (c *fakeReadCloser) Close() error {
	c.closeOnce.Do(func() {
		c.closed = true
		close(c.closedCh)
	})
	return nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
