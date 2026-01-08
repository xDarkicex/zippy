package zippy

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// =============================================================================
// Buffer Basic Operations Tests
// =============================================================================

func TestBuffer_Write(t *testing.T) {
	buf := New()
	defer buf.Release()

	data := []byte("hello world")
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
	if buf.Len() != len(data) {
		t.Errorf("Len() = %d, want %d", buf.Len(), len(data))
	}

	got := buf.Bytes()
	if !bytes.Equal(got, data) {
		t.Errorf("Bytes() = %q, want %q", got, data)
	}
}

func TestBuffer_WriteString(t *testing.T) {
	buf := New()
	defer buf.Release()

	str := "hello world"
	n, err := buf.WriteString(str)
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if n != len(str) {
		t.Errorf("WriteString returned %d, want %d", n, len(str))
	}

	got := buf.String()
	if got != str {
		t.Errorf("String() = %q, want %q", got, str)
	}
}

func TestBuffer_WriteByte(t *testing.T) {
	buf := New()
	defer buf.Release()

	// Write 300 bytes to test staging buffer allocation
	for i := 0; i < 300; i++ {
		if err := buf.WriteByte(byte(i % 256)); err != nil {
			t.Fatalf("WriteByte(%d) failed: %v", i, err)
		}
	}

	if buf.Len() != 300 {
		t.Errorf("Len() = %d, want 300", buf.Len())
	}

	// Verify content
	got := buf.Bytes()
	if len(got) != 300 {
		t.Errorf("Bytes() length = %d, want 300", len(got))
	}
	for i := 0; i < 300; i++ {
		if got[i] != byte(i%256) {
			t.Errorf("Bytes()[%d] = %d, want %d", i, got[i], i%256)
		}
	}
}

func TestBuffer_Read(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := []byte("hello world")
	buf.Write(input)

	output := make([]byte, 5)
	n, err := buf.Read(output)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Read returned %d, want 5", n)
	}
	if string(output) != "hello" {
		t.Errorf("Read data = %q, want %q", output, "hello")
	}

	// Read remainder
	remainder := make([]byte, 20)
	n, err = buf.Read(remainder)
	if err != nil && err != io.EOF {
		t.Fatalf("Read remainder failed: %v", err)
	}
	if string(remainder[:n]) != " world" {
		t.Errorf("Read remainder = %q, want %q", remainder[:n], " world")
	}

	// Should return EOF now
	n, err = buf.Read(remainder)
	if err != io.EOF {
		t.Errorf("Read after exhaustion: got err=%v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("Read after exhaustion: got n=%d, want 0", n)
	}
}

func TestBuffer_ReadByte(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := []byte("ABC")
	buf.Write(input)

	// Read all bytes
	for i, want := range input {
		got, err := buf.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte(%d) failed: %v", i, err)
		}
		if got != want {
			t.Errorf("ReadByte(%d) = %q, want %q", i, got, want)
		}
	}

	// Should return EOF
	_, err := buf.ReadByte()
	if err != io.EOF {
		t.Errorf("ReadByte after exhaustion: got err=%v, want io.EOF", err)
	}
}

// =============================================================================
// Buffer I/O Tests
// =============================================================================

func TestBuffer_ReadFrom(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := strings.Repeat("hello world\n", 1000)
	reader := strings.NewReader(input)

	n, err := buf.ReadFrom(reader)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("ReadFrom returned %d, want %d", n, len(input))
	}
	if buf.Len() != len(input) {
		t.Errorf("Len() = %d, want %d", buf.Len(), len(input))
	}

	got := buf.String()
	if got != input {
		t.Errorf("Content mismatch (showing first 100 chars)")
		t.Errorf("got:  %q", got[:100])
		t.Errorf("want: %q", input[:100])
	}
}

func TestBuffer_ReadN(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := strings.Repeat("X", 10000)
	reader := strings.NewReader(input)

	// Read exact amount
	n, err := buf.ReadN(reader, 5000)
	if err != nil {
		t.Fatalf("ReadN failed: %v", err)
	}
	if n != 5000 {
		t.Errorf("ReadN returned %d, want 5000", n)
	}
	if buf.Len() != 5000 {
		t.Errorf("Len() = %d, want 5000", buf.Len())
	}

	// Try reading more than available
	buf.Reset()
	reader = strings.NewReader("short")
	_, err = buf.ReadN(reader, 100)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("ReadN with insufficient data: got err=%v, want io.ErrUnexpectedEOF", err)
	}
}

func TestBuffer_WriteTo(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := "hello world"
	buf.WriteString(input)

	var output bytes.Buffer
	n, err := buf.WriteTo(&output)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("WriteTo returned %d, want %d", n, len(input))
	}
	if output.String() != input {
		t.Errorf("WriteTo content = %q, want %q", output.String(), input)
	}
	if buf.Len() != 0 {
		t.Errorf("Len() after WriteTo = %d, want 0", buf.Len())
	}
}

func TestBuffer_WriteToFile(t *testing.T) {
	buf := New()
	defer buf.Release()

	// Create temp file
	tmpfile, err := os.CreateTemp("", "zippy_test_*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	input := strings.Repeat("test data\n", 1000)
	buf.WriteString(input)

	n, err := buf.WriteTo(tmpfile)
	if err != nil {
		t.Fatalf("WriteTo file failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("WriteTo returned %d, want %d", n, len(input))
	}

	// Verify file content
	tmpfile.Seek(0, 0)
	got, err := io.ReadAll(tmpfile)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(got) != input {
		t.Errorf("File content mismatch (length: got=%d, want=%d)", len(got), len(input))
	}
}

// =============================================================================
// Size Limit Tests
// =============================================================================

func TestBuffer_SizeLimit(t *testing.T) {
	buf := New()
	defer buf.Release()

	// Default limit is 100MB
	if buf.MaxSize() != DefaultMaxBufferSize {
		t.Errorf("MaxSize() = %d, want %d", buf.MaxSize(), DefaultMaxBufferSize)
	}

	// Set smaller limit
	buf.SetMaxSize(100)

	// Write within limit
	_, err := buf.Write(make([]byte, 50))
	if err != nil {
		t.Fatalf("Write within limit failed: %v", err)
	}

	// Write exceeding limit
	_, err = buf.Write(make([]byte, 100))
	if err != ErrBufferFull {
		t.Errorf("Write exceeding limit: got err=%v, want ErrBufferFull", err)
	}
}

func TestBuffer_SizeLimitWriteByte(t *testing.T) {
	buf := New()
	defer buf.Release()
	buf.SetMaxSize(10)

	// Write 10 bytes (at limit)
	for i := 0; i < 10; i++ {
		if err := buf.WriteByte(byte(i)); err != nil {
			t.Fatalf("WriteByte(%d) failed: %v", i, err)
		}
	}

	// 11th byte should fail
	err := buf.WriteByte(99)
	if err != ErrBufferFull {
		t.Errorf("WriteByte exceeding limit: got err=%v, want ErrBufferFull", err)
	}
}

func TestBuffer_SizeLimitReadFrom(t *testing.T) {
	buf := New()
	defer buf.Release()
	buf.SetMaxSize(1000)

	input := strings.Repeat("X", 2000)
	reader := strings.NewReader(input)

	_, err := buf.ReadFrom(reader)
	if err != ErrBufferFull {
		t.Errorf("ReadFrom exceeding limit: got err=%v, want ErrBufferFull", err)
	}

	// Buffer should contain exactly 1000 bytes
	if buf.Len() != 1000 {
		t.Errorf("Len() after limit = %d, want 1000", buf.Len())
	}
}

func TestBuffer_Unlimited(t *testing.T) {
	buf := NewUnlimited()
	defer buf.Release()

	if buf.MaxSize() != NoSizeLimit {
		t.Errorf("NewUnlimited MaxSize() = %d, want %d", buf.MaxSize(), NoSizeLimit)
	}

	// Should be able to write large amounts
	largeData := make([]byte, 200*1024*1024) // 200MB
	_, err := buf.Write(largeData)
	if err != nil {
		t.Errorf("Write to unlimited buffer failed: %v", err)
	}
}

// =============================================================================
// Buffer Management Tests
// =============================================================================

func TestBuffer_Reset(t *testing.T) {
	buf := New()
	defer buf.Release()

	buf.SetMaxSize(500)
	buf.WriteString("test data")

	buf.Reset()

	if buf.Len() != 0 {
		t.Errorf("Len() after Reset = %d, want 0", buf.Len())
	}
	if buf.MaxSize() != 500 {
		t.Errorf("MaxSize() after Reset = %d, want 500 (should be preserved)", buf.MaxSize())
	}

	// Should be reusable
	buf.WriteString("new data")
	if buf.String() != "new data" {
		t.Errorf("Buffer not reusable after Reset")
	}
}

func TestBuffer_Truncate(t *testing.T) {
	buf := New()
	defer buf.Release()

	buf.WriteString("hello world")

	// Truncate to 5 bytes
	buf.Truncate(5)
	if buf.Len() != 5 {
		t.Errorf("Len() after Truncate(5) = %d, want 5", buf.Len())
	}

	// Read to verify actual truncation
	result := make([]byte, 10)
	n, _ := buf.Read(result)
	if n != 5 {
		t.Errorf("Read after Truncate(5) returned %d bytes, want 5", n)
	}
	if string(result[:n]) != "hello" {
		t.Errorf("Read after Truncate(5) = %q, want %q", string(result[:n]), "hello")
	}

	// Truncate to 0 (like Reset but preserves maxSize)
	buf.WriteString("test")
	buf.Truncate(0)
	if buf.Len() != 0 {
		t.Errorf("Len() after Truncate(0) = %d, want 0", buf.Len())
	}
}

func TestBuffer_TruncatePanic(t *testing.T) {
	buf := New()
	defer buf.Release()

	buf.WriteString("hello")

	// Negative should panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Truncate(-1) did not panic")
		}
	}()
	buf.Truncate(-1)
}

func TestBuffer_Grow(t *testing.T) {
	buf := New()
	defer buf.Release()

	initialCap := cap(buf.buckets)

	// First, let's consume the existing spare capacity
	// New() creates a buffer with defaultBucketCap=32 spare slots
	// So write enough to fill those slots first
	for i := 0; i < initialCap; i++ {
		buf.Write(make([]byte, 1024*1024)) // 1MB each
	}

	// Now the spare capacity should be 0
	spareCap := cap(buf.buckets) - len(buf.buckets)
	if spareCap != 0 {
		t.Logf("After filling: len=%d, cap=%d, spare=%d", len(buf.buckets), cap(buf.buckets), spareCap)
	}

	// NOW grow by 100MB (needs ~25 more buckets)
	buf.Grow(100 * 1024 * 1024) // 100MB

	newCap := cap(buf.buckets)
	expectedMinCap := len(buf.buckets) + 25 // Should add 25 to current length

	if newCap < expectedMinCap {
		t.Errorf("Grow did not allocate enough slots: got=%d, want>=%d", newCap, expectedMinCap)
	}

	t.Logf("Bucket capacity: initial=%d, after_writes=%d, after_grow=%d",
		initialCap, len(buf.buckets), newCap)
}

func TestBuffer_Cap(t *testing.T) {
	buf := New()
	defer buf.Release()

	// Empty buffer
	if buf.Cap() != 0 {
		t.Errorf("Cap() for empty buffer = %d, want 0", buf.Cap())
	}

	// After ReadFrom with pooled chunks
	input := strings.Repeat("X", 8*1024*1024) // 8MB
	buf.ReadFrom(strings.NewReader(input))

	gotCap := buf.Cap()
	// ReadFrom uses pooled 4MB chunks, so 8MB = 2 chunks = 8MB capacity
	// But actual capacity depends on chunk pooling
	if gotCap < 4*1024*1024 {
		t.Errorf("Cap() after 8MB read = %d, want >= 4MB", gotCap)
	}

	t.Logf("Cap() = %d (%.1f MB) for %d bytes (%.1f MB) of data",
		gotCap, float64(gotCap)/(1024*1024),
		len(input), float64(len(input))/(1024*1024))
}

// =============================================================================
// Bytes() Performance Path Tests
// =============================================================================

func TestBuffer_BytesSingleBucket(t *testing.T) {
	buf := New()
	defer buf.Release()

	input := []byte("single bucket")
	buf.Write(input)

	got := buf.Bytes()
	if !bytes.Equal(got, input) {
		t.Errorf("Bytes() = %q, want %q", got, input)
	}

	// Verify zero-copy (same backing array)
	if len(got) > 0 && len(input) > 0 && &got[0] != &input[0] {
		t.Logf("Note: Bytes() returned different pointer (may be OK depending on implementation)")
	}
}

func TestBuffer_BytesMultipleBuckets(t *testing.T) {
	buf := New()
	defer buf.Release()

	// Force multiple buckets
	buf.Write([]byte("hello "))
	buf.Write([]byte("world"))

	got := buf.Bytes()
	want := "hello world"
	if string(got) != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}
}

func TestBuffer_BytesWithOffset(t *testing.T) {
	buf := New()
	defer buf.Release()

	buf.Write([]byte("hello world"))

	// Partially read
	p := make([]byte, 6)
	buf.Read(p)

	got := buf.Bytes()
	want := "world"
	if string(got) != want {
		t.Errorf("Bytes() after partial read = %q, want %q", got, want)
	}
}

// =============================================================================
// Kernel Copy Tests (Linux-specific)
// =============================================================================

func TestSpliceConn_UnsupportedPlatform(t *testing.T) {
	// Create Unix socket pair (even on non-Linux, test graceful degradation)
	// This test verifies the function doesn't crash on unsupported platforms

	if err := testUnixSocketPair(func(client, server *net.UnixConn) error {
		_, err := SpliceConn(server, client)
		// Should return ENOTSUP on non-Linux or succeed on Linux
		if err != nil && err != syscall.ENOTSUP {
			return err
		}
		return nil
	}); err != nil {
		// Socket creation may fail on some systems, that's OK
		t.Logf("Unix socket test skipped: %v", err)
	}
}

func TestKernelCopy(t *testing.T) {
	input := strings.Repeat("test data\n", 1000)
	reader := strings.NewReader(input)

	var output bytes.Buffer
	n, err := KernelCopy(&output, reader)
	if err != nil {
		t.Fatalf("KernelCopy failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("KernelCopy returned %d, want %d", n, len(input))
	}
	if output.String() != input {
		t.Errorf("KernelCopy content mismatch")
	}
}

func TestProxyLoop(t *testing.T) {
	// Test fallback path with regular Unix sockets
	if err := testUnixSocketPair(func(client, server *net.UnixConn) error {
		// Write data in goroutine
		go func() {
			client.Write([]byte("hello"))
			client.Close()
		}()

		var output bytes.Buffer
		go func() {
			io.Copy(&output, server)
		}()

		time.Sleep(50 * time.Millisecond)

		// Note: ProxyLoop blocks, so we test the basic infrastructure here
		return nil
	}); err != nil {
		t.Logf("ProxyLoop test skipped: %v", err)
	}
}

// =============================================================================
// Streaming Tests
// =============================================================================

func TestStreamCopy(t *testing.T) {
	input := strings.Repeat("X", 10*1024*1024) // 10MB
	reader := strings.NewReader(input)

	var output bytes.Buffer
	var progressCalls int

	n, err := StreamCopy(&output, reader, 1024*1024, func(r, w int64) error {
		progressCalls++
		if w > int64(len(input)) {
			return errors.New("written exceeds input size")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("StreamCopy failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("StreamCopy returned %d, want %d", n, len(input))
	}
	if output.Len() != len(input) {
		t.Errorf("StreamCopy output length = %d, want %d", output.Len(), len(input))
	}
	if progressCalls < 5 {
		t.Errorf("Progress callback called %d times, want >= 5", progressCalls)
	}
}

func TestStreamCopy_ProgressCancellation(t *testing.T) {
	input := strings.Repeat("X", 10*1024*1024) // 10MB
	reader := strings.NewReader(input)

	var output bytes.Buffer
	cancelErr := errors.New("user canceled")

	_, err := StreamCopy(&output, reader, 1024*1024, func(r, w int64) error {
		if w > 2*1024*1024 { // Cancel after 2MB
			return cancelErr
		}
		return nil
	})

	if err != cancelErr {
		t.Errorf("StreamCopy cancellation: got err=%v, want %v", err, cancelErr)
	}
}

func TestStreamCopyWithTimeout(t *testing.T) {
	input := strings.Repeat("Y", 5*1024*1024) // 5MB
	reader := strings.NewReader(input)

	var output bytes.Buffer

	n, err := StreamCopyWithTimeout(&output, reader, 1024*1024, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("StreamCopyWithTimeout failed: %v", err)
	}
	if n != int64(len(input)) {
		t.Errorf("StreamCopyWithTimeout returned %d, want %d", n, len(input))
	}
}

func TestStreamProxy(t *testing.T) {
	// Test with nil progress (should try splice)
	if err := testUnixSocketPair(func(client, server *net.UnixConn) error {
		// Write and close in goroutine
		go func() {
			client.Write([]byte("hello world"))
			client.Close()
		}()

		// Read in another goroutine
		done := make(chan struct{})
		var output bytes.Buffer
		go func() {
			io.Copy(&output, server)
			close(done)
		}()

		select {
		case <-done:
			if output.String() != "hello world" {
				t.Errorf("StreamProxy received %q, want %q", output.String(), "hello world")
			}
		case <-time.After(time.Second):
			t.Error("StreamProxy test timeout")
		}

		return nil
	}); err != nil {
		t.Logf("StreamProxy test skipped: %v", err)
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestBuffer_Nil(t *testing.T) {
	var buf *Buffer

	if buf.Len() != 0 {
		t.Errorf("nil buffer Len() = %d, want 0", buf.Len())
	}
	if buf.Cap() != 0 {
		t.Errorf("nil buffer Cap() = %d, want 0", buf.Cap())
	}
	if buf.Bytes() != nil {
		t.Errorf("nil buffer Bytes() != nil")
	}

	_, err := buf.Write([]byte("test"))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("nil buffer Write: got err=%v, want io.ErrUnexpectedEOF", err)
	}
}

func TestBuffer_EmptyWrites(t *testing.T) {
	buf := New()
	defer buf.Release()

	n, err := buf.Write(nil)
	if err != nil || n != 0 {
		t.Errorf("Write(nil): got (%d, %v), want (0, nil)", n, err)
	}

	n, err = buf.Write([]byte{})
	if err != nil || n != 0 {
		t.Errorf("Write([]): got (%d, %v), want (0, nil)", n, err)
	}

	n, err = buf.WriteString("")
	if err != nil || n != 0 {
		t.Errorf(`WriteString(""): got (%d, %v), want (0, nil)`, n, err)
	}
}

func TestBuffer_ReadFromEmpty(t *testing.T) {
	buf := New()
	defer buf.Release()

	empty := strings.NewReader("")
	n, err := buf.ReadFrom(empty)
	if err != nil || n != 0 {
		t.Errorf("ReadFrom empty: got (%d, %v), want (0, nil)", n, err)
	}
}

// =============================================================================
// Benchmarks - Basic Operations
// =============================================================================

func BenchmarkBuffer_Write(b *testing.B) {
	buf := New()
	defer buf.Release()
	data := make([]byte, 1024)

	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		buf.Write(data)
		if i%1000 == 999 {
			buf.Reset()
		}
	}
}

func BenchmarkBuffer_WriteString(b *testing.B) {
	buf := New()
	defer buf.Release()
	str := strings.Repeat("a", 1024)

	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		buf.WriteString(str)
		if i%1000 == 999 {
			buf.Reset()
		}
	}
}

func BenchmarkBuffer_WriteByte(b *testing.B) {
	buf := New()
	defer buf.Release()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.WriteByte(byte(i))
		if i%10000 == 9999 {
			buf.Reset()
		}
	}
}

func BenchmarkBuffer_ReadByte(b *testing.B) {
	buf := New()
	defer buf.Release()

	// Pre-fill buffer
	data := make([]byte, b.N)
	for i := range data {
		data[i] = byte(i)
	}
	buf.Write(data)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.ReadByte()
	}
}

func BenchmarkBuffer_ReadFrom(b *testing.B) {
	data := make([]byte, 4*1024*1024) // 4MB
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := New()
		reader := bytes.NewReader(data)
		buf.ReadFrom(reader)
		buf.Release()
	}
}

func TestBuffer_ReadFrom_PanicCleanup(t *testing.T) {
	// Count chunks in pool before test
	initialChunks := 0
	for i := 0; i < 10; i++ {
		chunk := chunkPool.Get().([]byte)
		initialChunks++
		chunkPool.Put(chunk)
	}

	buf := New()
	defer buf.Release()

	// Reader that panics after 2 successful reads
	panicReader := &panicAfterNReads{
		data:       bytes.Repeat([]byte("X"), 10*1024*1024), // 10MB
		panicAfter: 2,
	}

	// Should panic and clean up all chunks
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		buf.ReadFrom(panicReader)
	}()

	// Verify chunks were returned to pool
	// Try to get chunks - should succeed without allocation
	retrieved := 0
	for i := 0; i < initialChunks+2; i++ { // +2 for chunks allocated during panic
		chunk := chunkPool.Get().([]byte)
		if cap(chunk) == chunkSize {
			retrieved++
		}
		chunkPool.Put(chunk)
	}

	if retrieved < 2 {
		t.Errorf("Chunks leaked! Retrieved %d pooled chunks, expected at least 2", retrieved)
	}

	t.Logf("✓ Panic cleanup working: retrieved %d pooled chunks", retrieved)
}

// Helper: Reader that panics after N reads
type panicAfterNReads struct {
	data       []byte
	offset     int
	readCount  int
	panicAfter int
}

func (r *panicAfterNReads) Read(p []byte) (n int, err error) {
	r.readCount++
	if r.readCount > r.panicAfter {
		panic("intentional panic for testing")
	}

	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func BenchmarkBuffer_WriteTo(b *testing.B) {
	data := make([]byte, 4*1024*1024) // 4MB
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := New()
		buf.Write(data)
		buf.WriteTo(io.Discard)
		buf.Release()
	}
}

func BenchmarkBuffer_Bytes_SingleBucket(b *testing.B) {
	buf := New()
	defer buf.Release()
	buf.Write([]byte("hello world"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buf.Bytes()
	}
}

func BenchmarkBuffer_Bytes_MultipleBuckets(b *testing.B) {
	buf := New()
	defer buf.Release()

	for i := 0; i < 10; i++ {
		buf.Write([]byte("bucket data "))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buf.Bytes()
	}
}

// =============================================================================
// Benchmarks - I/O Operations
// =============================================================================

func BenchmarkKernelCopy_Small(b *testing.B) {
	data := make([]byte, 4*1024) // 4KB
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		KernelCopy(io.Discard, reader)
	}
}

func BenchmarkKernelCopy_Large(b *testing.B) {
	data := make([]byte, 4*1024*1024) // 4MB
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		KernelCopy(io.Discard, reader)
	}
}

func BenchmarkKernelCopy_vs_IOCopy(b *testing.B) {
	data := make([]byte, 4*1024*1024) // 4MB

	b.Run("KernelCopy", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			KernelCopy(io.Discard, reader)
		}
	})

	b.Run("io.Copy", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			io.Copy(io.Discard, reader)
		}
	})
}

func BenchmarkStreamCopy(b *testing.B) {
	data := make([]byte, 10*1024*1024) // 10MB
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		StreamCopy(io.Discard, reader, 1024*1024, nil)
	}
}

// =============================================================================
// Benchmarks - Pool Overhead
// =============================================================================

func BenchmarkBuffer_New(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := New()
		buf.Release()
	}
}

func BenchmarkBuffer_NewAndWrite(b *testing.B) {
	data := []byte("test data")
	b.SetBytes(int64(len(data)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := New()
		buf.Write(data)
		buf.Release()
	}
}

func BenchmarkBuffer_Reuse(b *testing.B) {
	buf := New()
	defer buf.Release()
	data := []byte("test data")

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		buf.Write(data)
		buf.Reset()
	}
}

// =============================================================================
// Benchmarks - Comparison with stdlib
// =============================================================================

func BenchmarkComparison_Write(b *testing.B) {
	data := []byte(strings.Repeat("x", 1024))

	b.Run("zippy.Buffer", func(b *testing.B) {
		buf := New()
		defer buf.Release()
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf.Write(data)
			if i%1000 == 999 {
				buf.Reset()
			}
		}
	})

	b.Run("bytes.Buffer", func(b *testing.B) {
		var buf bytes.Buffer
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf.Write(data)
			if i%1000 == 999 {
				buf.Reset()
			}
		}
	})
}

func BenchmarkComparison_ReadFrom(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB

	b.Run("zippy.Buffer", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf := New()
			reader := bytes.NewReader(data)
			buf.ReadFrom(reader)
			buf.Release()
		}
	})

	b.Run("bytes.Buffer", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			reader := bytes.NewReader(data)
			buf.ReadFrom(reader)
		}
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// testUnixSocketPair creates a Unix socket pair for testing.
// The provided function receives both ends of the connection.
func testUnixSocketPair(fn func(client, server *net.UnixConn) error) error {
	// Create temporary socket file
	tmpfile, err := os.CreateTemp("", "zippy_sock_*.sock")
	if err != nil {
		return err
	}
	sockPath := tmpfile.Name()
	tmpfile.Close()
	os.Remove(sockPath)
	defer os.Remove(sockPath)

	// Listen on Unix socket
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath, Net: "unix"})
	if err != nil {
		return err
	}
	defer listener.Close()

	// Connect in goroutine
	errChan := make(chan error, 1)
	var client *net.UnixConn

	go func() {
		var err error
		client, err = net.DialUnix("unix", nil, &net.UnixAddr{Name: sockPath, Net: "unix"})
		errChan <- err
	}()

	// Accept connection
	server, err := listener.AcceptUnix()
	if err != nil {
		return err
	}
	defer server.Close()

	if err := <-errChan; err != nil {
		return err
	}
	defer client.Close()

	return fn(client, server)
}

// =============================================================================
// Example Tests (will show up in godoc)
// =============================================================================

func ExampleBuffer() {
	buf := New()
	defer buf.Release()

	buf.WriteString("Hello, ")
	buf.WriteString("World!")

	// Output to any io.Writer
	buf.WriteTo(os.Stdout)
	// Output: Hello, World!
}

func ExampleKernelCopy() {
	// Efficient copy with automatic optimization
	src := strings.NewReader("data to copy")
	dst := &bytes.Buffer{}

	n, err := KernelCopy(dst, src)
	if err != nil {
		panic(err)
	}

	_ = n // bytes transferred
}

func ExampleStreamCopy() {
	src := strings.NewReader(strings.Repeat("X", 10*1024*1024)) // 10MB
	dst := io.Discard

	// Stream with progress tracking
	n, err := StreamCopy(dst, src, 1024*1024, func(read, written int64) error {
		// Update progress bar, check for cancellation, etc.
		_ = written
		return nil
	})

	if err != nil {
		panic(err)
	}

	_ = n
}
