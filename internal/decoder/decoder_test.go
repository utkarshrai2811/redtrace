package decoder

import (
	"bytes"
	"strings"
	"testing"
)

func TestApply_RoundTrips(t *testing.T) {
	tests := []struct {
		name       string
		enc, dec   Op
		input      string
		wantEncode string
	}{
		{"url", URLEncode, URLDecode, "a b&c=d", "a+b%26c%3Dd"},
		{"base64", Base64Encode, Base64Decode, "hello", "aGVsbG8="},
		{"base64url", Base64URLEncode, Base64URLDecode, "hello", "aGVsbG8"},
		{"hex", HexEncode, HexDecode, "hi", "6869"},
		{"html", HTMLEncode, HTMLDecode, `<a href="x">`, "&lt;a href=&#34;x&#34;&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := Apply(tt.enc, []byte(tt.input))
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if string(enc) != tt.wantEncode {
				t.Errorf("encode = %q, want %q", enc, tt.wantEncode)
			}
			dec, err := Apply(tt.dec, enc)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if string(dec) != tt.input {
				t.Errorf("round-trip = %q, want %q", dec, tt.input)
			}
		})
	}
}

func TestApply_Gzip(t *testing.T) {
	input := []byte(strings.Repeat("redtrace ", 50))
	compressed, err := Apply(GzipCompress, input)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(compressed) >= len(input) {
		t.Errorf("gzip did not shrink repetitive input: %d >= %d", len(compressed), len(input))
	}
	out, err := Apply(GzipDecompress, compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(out) != string(input) {
		t.Error("gzip round-trip mismatch")
	}
}

func TestApply_JWTDecode(t *testing.T) {
	// {"alg":"HS256","typ":"JWT"} . {"sub":"1234","admin":true} . sig
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0IiwiYWRtaW4iOnRydWV9.sig"
	out, err := Apply(JWTDecode, []byte(jwt))
	if err != nil {
		t.Fatalf("jwt decode: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"alg": "HS256"`) || !strings.Contains(s, `"admin": true`) {
		t.Errorf("jwt decode missing claims: %s", s)
	}

	if _, err := Apply(JWTDecode, []byte("notajwt")); err == nil {
		t.Error("expected error for non-JWT")
	}
}

func TestChain(t *testing.T) {
	// URL-encode then Base64-encode, then reverse.
	out, err := Chain([]Op{URLEncode, Base64Encode}, []byte("a b"))
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	back, err := Chain([]Op{Base64Decode, URLDecode}, out)
	if err != nil {
		t.Fatalf("chain back: %v", err)
	}
	if string(back) != "a b" {
		t.Errorf("chain round-trip = %q", back)
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		input string
		want  Op
		ok    bool
	}{
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.sig", JWTDecode, true},
		{"hello%20world", URLDecode, true},
		{"68656c6c6f", HexDecode, true},
		{"aGVsbG8gd29ybGQ=", Base64Decode, true},
		{"plain text here", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := Detect([]byte(tt.input))
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("Detect(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestGzipDecompress_RejectsBomb(t *testing.T) {
	// Lower the cap so the test stays cheap; a blob whose output exceeds it must
	// be rejected rather than buffered into memory.
	orig := maxDecompressed
	maxDecompressed = 1024
	defer func() { maxDecompressed = orig }()

	bomb, err := Apply(GzipCompress, bytes.Repeat([]byte("A"), 64*1024))
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if _, err := Apply(GzipDecompress, bomb); err == nil {
		t.Fatal("expected oversized gzip to be rejected, got nil error")
	}

	// A small payload under the cap still round-trips.
	small, _ := Apply(GzipCompress, []byte("hello"))
	out, err := Apply(GzipDecompress, small)
	if err != nil || string(out) != "hello" {
		t.Fatalf("small gzip round-trip: out=%q err=%v", out, err)
	}
}
