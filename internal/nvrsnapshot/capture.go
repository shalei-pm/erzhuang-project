package nvrsnapshot

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"io"
	"os/exec"
	"strings"

	"nhooyr.io/websocket"
)

const (
	maxJPEGBytes     = 1 << 20
	maxAnnexBBytes   = 8 << 20
	maxThumbnailEdge = 640
)

const (
	ErrorAuthorizationFailed ErrorCode = "authorization_failed"
	ErrorWSSConnectFailed    ErrorCode = "wss_connect_failed"
	ErrorWSSConnectTimeout   ErrorCode = "wss_connect_timeout"
	ErrorDecodeFailed        ErrorCode = "decode_failed"
	ErrorThumbnailInvalid    ErrorCode = "thumbnail_invalid"

	// Deprecated aliases preserve source compatibility while retaining the new,
	// more specific external error code values.
	ErrorWSSFailed  = ErrorWSSConnectFailed
	ErrorWSSTimeout = ErrorWSSConnectTimeout
)

type StreamMode string

const (
	StreamModeLive     StreamMode = "live"
	StreamModePlayback StreamMode = "playback"
)

type StreamRequest struct {
	Mode      StreamMode
	StartTime int64
	EndTime   int64
}

// StreamAuthorizer obtains a short-lived WSS URL for one camera. The URL is
// an internal hand-off only and must never be persisted or exposed to callers.
type StreamAuthorizer interface {
	AuthorizeStream(ctx context.Context, cameraID int64, request StreamRequest) (wssURL string, code ErrorCode)
}

// JPEG is the only media result this package exposes. Bytes contains one
// bounded JPEG thumbnail, never RTP, Annex-B media, an upstream body, or URL.
type JPEG struct {
	Bytes       []byte
	Width       int
	Height      int
	ContentType string
}

type JPEGCapture interface {
	Capture(ctx context.Context, wssURL string) (JPEG, ErrorCode)
}

// CaptureService joins authorization and media capture without retaining the
// temporary URL. It whitelists dependency codes before they reach callers.
type CaptureService struct {
	authorizer StreamAuthorizer
	capture    JPEGCapture
}

func NewCaptureService(authorizer StreamAuthorizer, capture JPEGCapture) *CaptureService {
	return &CaptureService{authorizer: authorizer, capture: capture}
}

func (s *CaptureService) Capture(ctx context.Context, cameraID int64, request StreamRequest) (JPEG, ErrorCode) {
	if s == nil || s.authorizer == nil || s.capture == nil {
		return JPEG{}, ErrorAuthorizationFailed
	}
	wssURL, code := s.authorizer.AuthorizeStream(ctx, cameraID, request)
	if code != "" {
		if code == ErrorAuthorizationFailed {
			return JPEG{}, code
		}
		return JPEG{}, ErrorAuthorizationFailed
	}
	if strings.TrimSpace(wssURL) == "" {
		return JPEG{}, ErrorAuthorizationFailed
	}
	jpeg, code := s.capture.Capture(ctx, wssURL)
	return jpeg, whitelistCaptureCode(code)
}

func whitelistCaptureCode(code ErrorCode) ErrorCode {
	switch code {
	case "", ErrorAuthorizationFailed, ErrorWSSConnectFailed, ErrorWSSConnectTimeout, ErrorMediaTimeout, ErrorDemuxFailed, ErrorDecodeFailed, ErrorThumbnailInvalid:
		return code
	default:
		return ErrorDecodeFailed
	}
}

type WebSocketMessageType uint8

const (
	WebSocketMessageText   WebSocketMessageType = 1
	WebSocketMessageBinary WebSocketMessageType = 2
)

type WebSocketMessage struct {
	Type WebSocketMessageType
	Data []byte
}

type WebSocketConn interface {
	Read(ctx context.Context) (WebSocketMessage, error)
	Close() error
}

type WebSocketDialer interface {
	Dial(ctx context.Context, wssURL string) (WebSocketConn, error)
}

// NhooyrWebSocketDialer is the production WSS connector. It does not retain,
// log, or return stream URLs; callers hand each URL directly to Dial.
type NhooyrWebSocketDialer struct {
	Options *websocket.DialOptions
}

func (d NhooyrWebSocketDialer) Dial(ctx context.Context, wssURL string) (WebSocketConn, error) {
	connection, _, err := websocket.Dial(ctx, wssURL, d.Options)
	if err != nil {
		return nil, err
	}
	return nhooyrWebSocketConn{connection: connection}, nil
}

type nhooyrWebSocketConn struct {
	connection *websocket.Conn
}

func (c nhooyrWebSocketConn) Read(ctx context.Context) (WebSocketMessage, error) {
	messageType, data, err := c.connection.Read(ctx)
	if err != nil {
		return WebSocketMessage{}, err
	}
	if messageType != websocket.MessageBinary {
		return WebSocketMessage{}, nil
	}
	return WebSocketMessage{Type: WebSocketMessageBinary, Data: data}, nil
}

func (c nhooyrWebSocketConn) Close() error {
	return c.connection.Close(websocket.StatusNormalClosure, "")
}

type FFmpegCommand interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
	Kill() error
}

type CommandFactory interface {
	New(ctx context.Context, args ...string) (FFmpegCommand, error)
}

// ExecCommandFactory creates real ffmpeg commands without invoking a shell.
type ExecCommandFactory struct {
	Path string
}

func (f ExecCommandFactory) New(ctx context.Context, args ...string) (FFmpegCommand, error) {
	path := strings.TrimSpace(f.Path)
	if path == "" {
		path = "ffmpeg"
	}
	return execFFmpegCommand{command: exec.CommandContext(ctx, path, args...)}, nil
}

type execFFmpegCommand struct {
	command *exec.Cmd
}

func (c execFFmpegCommand) StdinPipe() (io.WriteCloser, error) { return c.command.StdinPipe() }
func (c execFFmpegCommand) StdoutPipe() (io.ReadCloser, error) { return c.command.StdoutPipe() }
func (c execFFmpegCommand) Start() error                       { return c.command.Start() }
func (c execFFmpegCommand) Wait() error                        { return c.command.Wait() }
func (c execFFmpegCommand) Kill() error                        { return c.command.Process.Kill() }

// WebSocketJPEGCapture consumes binary RTP/H.265 messages. It sends ffmpeg
// only the retained parameter sets plus a complete, marker-delimited key AU.
type WebSocketJPEGCapture struct {
	dialer         WebSocketDialer
	commandFactory CommandFactory
}

func NewWebSocketJPEGCapture(dialer WebSocketDialer, commandFactory CommandFactory) *WebSocketJPEGCapture {
	return &WebSocketJPEGCapture{dialer: dialer, commandFactory: commandFactory}
}

func (c *WebSocketJPEGCapture) Capture(ctx context.Context, wssURL string) (JPEG, ErrorCode) {
	if c == nil || c.dialer == nil {
		return JPEG{}, ErrorWSSConnectFailed
	}
	connection, err := c.dialer.Dial(ctx, wssURL)
	if err != nil || connection == nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return JPEG{}, ErrorWSSConnectTimeout
		}
		return JPEG{}, ErrorWSSConnectFailed
	}
	defer connection.Close()

	depacketizer := NewDepacketizer()
	parameterSets := [3][]byte{}
	defer clearParameterSets(&parameterSets)

	var accessUnit accessUnit
	defer accessUnit.wipe()
	for {
		message, err := connection.Read(ctx)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return JPEG{}, ErrorMediaTimeout
			}
			return JPEG{}, ErrorWSSConnectFailed
		}
		if message.Type != WebSocketMessageBinary {
			return JPEG{}, ErrorDemuxFailed
		}

		result, code := depacketizer.FeedRTPWithMetadata(message.Data)
		if code != "" {
			return JPEG{}, code
		}
		if len(result.NAL) == 0 {
			continue
		}

		nalType := h265NALType(result.NAL)
		if index := parameterSetIndex(nalType); index >= 0 {
			if len(result.NAL) > maxAnnexBBytes {
				return JPEG{}, ErrorDemuxFailed
			}
			parameterSets[index] = replaceParameterSet(parameterSets[index], result.NAL)
			// Parameter-set packets are retained separately and never terminate an
			// in-progress access unit, even when their RTP marker is set.
			continue
		}
		if !accessUnit.acceptsTimestamp(result.Timestamp) {
			accessUnit.reset()
		}
		if len(result.NAL) > maxAnnexBBytes-len(accessUnit.bytes) {
			return JPEG{}, ErrorDemuxFailed
		}
		accessUnit.append(result.Timestamp, result.NAL, isH265KeyFrame(result.NAL))
		if !result.Marker {
			continue
		}
		if accessUnit.hasKeyFrame && hasAllParameterSets(parameterSets) {
			media, ok := combineAccessUnit(parameterSets, accessUnit.bytes)
			if !ok {
				return JPEG{}, ErrorDemuxFailed
			}
			jpeg, code := c.captureJPEG(ctx, media)
			clear(media)
			return jpeg, code
		}
		accessUnit.reset()
	}
}

type accessUnit struct {
	timestamp   uint32
	active      bool
	hasKeyFrame bool
	bytes       []byte
}

func (a *accessUnit) acceptsTimestamp(timestamp uint32) bool {
	return !a.active || a.timestamp == timestamp
}

func (a *accessUnit) append(timestamp uint32, nal []byte, keyFrame bool) {
	if !a.active {
		a.active = true
		a.timestamp = timestamp
	}
	a.bytes = append(a.bytes, nal...)
	a.hasKeyFrame = a.hasKeyFrame || keyFrame
}

func (a *accessUnit) reset() {
	clear(a.bytes)
	a.bytes = a.bytes[:0]
	a.active = false
	a.hasKeyFrame = false
	a.timestamp = 0
}

func (a *accessUnit) wipe() {
	clear(a.bytes[:cap(a.bytes)])
	a.bytes = nil
	a.active = false
	a.hasKeyFrame = false
	a.timestamp = 0
}

func parameterSetIndex(nalType byte) int {
	switch nalType {
	case 32:
		return 0
	case 33:
		return 1
	case 34:
		return 2
	default:
		return -1
	}
}

func clearParameterSets(parameterSets *[3][]byte) {
	for index := range parameterSets {
		clear(parameterSets[index][:cap(parameterSets[index])])
		parameterSets[index] = nil
	}
}

func replaceParameterSet(old, next []byte) []byte {
	clear(old[:cap(old)])
	return append(old[:0], next...)
}

func hasAllParameterSets(parameterSets [3][]byte) bool {
	for _, parameterSet := range parameterSets {
		if len(parameterSet) == 0 {
			return false
		}
	}
	return true
}

func combineAccessUnit(parameterSets [3][]byte, accessUnit []byte) ([]byte, bool) {
	total := len(accessUnit)
	for _, parameterSet := range parameterSets {
		if len(parameterSet) > maxAnnexBBytes-total {
			return nil, false
		}
		total += len(parameterSet)
	}
	if total == 0 {
		return nil, false
	}
	media := make([]byte, 0, total)
	for _, parameterSet := range parameterSets {
		media = append(media, parameterSet...)
	}
	media = append(media, accessUnit...)
	return media, true
}

func h265NALType(annexBNAL []byte) byte {
	if len(annexBNAL) < 6 || !bytes.Equal(annexBNAL[:4], []byte{0, 0, 0, 1}) {
		return 0xff
	}
	return (annexBNAL[4] >> 1) & 0x3f
}

func isH265KeyFrame(annexBNAL []byte) bool {
	nalType := h265NALType(annexBNAL)
	return nalType >= 16 && nalType <= 21
}

func (c *WebSocketJPEGCapture) captureJPEG(ctx context.Context, annexBMedia []byte) (JPEG, ErrorCode) {
	if c.commandFactory == nil {
		return JPEG{}, ErrorDecodeFailed
	}
	command, err := c.commandFactory.New(ctx, ffmpegArguments...)
	if err != nil || command == nil {
		return JPEG{}, ErrorDecodeFailed
	}
	stdin, err := command.StdinPipe()
	if err != nil || stdin == nil {
		return JPEG{}, ErrorDecodeFailed
	}
	stdout, err := command.StdoutPipe()
	if err != nil || stdout == nil {
		_ = stdin.Close()
		return JPEG{}, ErrorDecodeFailed
	}

	type readResult struct {
		data     []byte
		tooLarge bool
		err      error
	}
	var (
		started       bool
		waited        bool
		readDone      chan readResult
		writeDone     chan error
		readFinished  bool
		writeFinished bool
	)
	shutdown := func() {
		_ = stdin.Close()
		_ = stdout.Close()
		if started && !waited {
			_ = command.Kill()
			_ = command.Wait()
			waited = true
		}
		if readDone != nil && !readFinished {
			<-readDone
			readFinished = true
		}
		if writeDone != nil && !writeFinished {
			<-writeDone
			writeFinished = true
		}
	}
	defer shutdown()

	if err := command.Start(); err != nil {
		return JPEG{}, ErrorDecodeFailed
	}
	started = true

	readDone = make(chan readResult, 1)
	go func() {
		data, tooLarge, err := readBoundedJPEG(stdout)
		readDone <- readResult{data: data, tooLarge: tooLarge, err: err}
	}()
	writeDone = make(chan error, 1)
	go func() {
		_, err := stdin.Write(annexBMedia)
		writeDone <- err
	}()

	var output []byte
	succeeded := false
	defer func() {
		if !succeeded {
			wipeBytes(output)
		}
	}()
	for !readFinished || !writeFinished {
		select {
		case result := <-readDone:
			readFinished = true
			if result.tooLarge {
				wipeBytes(result.data)
				return JPEG{}, ErrorThumbnailInvalid
			}
			if result.err != nil {
				wipeBytes(result.data)
				return JPEG{}, ErrorDecodeFailed
			}
			output = result.data
		case err := <-writeDone:
			writeFinished = true
			if err != nil {
				return JPEG{}, ErrorDecodeFailed
			}
			if err := stdin.Close(); err != nil {
				return JPEG{}, ErrorDecodeFailed
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return JPEG{}, ErrorMediaTimeout
			}
			return JPEG{}, ErrorDecodeFailed
		}
	}
	if err := command.Wait(); err != nil {
		waited = true
		return JPEG{}, ErrorDecodeFailed
	}
	waited = true
	jpeg, code := validateJPEG(output)
	if code != "" {
		return JPEG{}, code
	}
	succeeded = true
	return jpeg, ""
}

var ffmpegArguments = []string{
	"-hide_banner", "-loglevel", "error", "-f", "hevc", "-i", "pipe:0",
	"-frames:v", "1", "-vf", "scale=640:640:force_original_aspect_ratio=decrease", "-q:v", "5", "-f", "image2", "pipe:1",
}

func readBoundedJPEG(reader io.Reader) ([]byte, bool, error) {
	var output bytes.Buffer
	buffer := make([]byte, 32<<10)
	defer wipeBytes(buffer)
	for {
		read, err := reader.Read(buffer)
		if read > 0 {
			if read > maxJPEGBytes-output.Len() {
				wipeBytes(output.Bytes())
				return nil, true, nil
			}
			_, _ = output.Write(buffer[:read])
		}
		if err == io.EOF {
			return output.Bytes(), false, nil
		}
		if err != nil {
			wipeBytes(output.Bytes())
			return nil, false, err
		}
	}
}

func validateJPEG(data []byte) (JPEG, ErrorCode) {
	if len(data) == 0 {
		return JPEG{}, ErrorDecodeFailed
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		wipeBytes(data)
		return JPEG{}, ErrorDecodeFailed
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		wipeBytes(data)
		return JPEG{}, ErrorDecodeFailed
	}
	if width > maxThumbnailEdge || height > maxThumbnailEdge || len(data) > maxJPEGBytes {
		wipeBytes(data)
		return JPEG{}, ErrorThumbnailInvalid
	}
	return JPEG{Bytes: data, Width: width, Height: height, ContentType: "image/jpeg"}, ""
}

func wipeBytes(data []byte) {
	clear(data[:cap(data)])
}
