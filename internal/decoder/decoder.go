// Package decoder implements the encode/decode transformations behind the
// RedTrace Decoder tool. Operations are composable into a chain.
package decoder

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"strings"
)

// Op is a single decoder transformation.
type Op string

const (
	URLEncode       Op = "url_encode"
	URLDecode       Op = "url_decode"
	Base64Encode    Op = "base64_encode"
	Base64Decode    Op = "base64_decode"
	Base64URLEncode Op = "base64url_encode"
	Base64URLDecode Op = "base64url_decode"
	HexEncode       Op = "hex_encode"
	HexDecode       Op = "hex_decode"
	HTMLEncode      Op = "html_encode"
	HTMLDecode      Op = "html_decode"
	GzipCompress    Op = "gzip_compress"
	GzipDecompress  Op = "gzip_decompress"
	JWTDecode       Op = "jwt_decode"
)

// Apply runs a single operation against input.
func Apply(op Op, input []byte) ([]byte, error) {
	switch op {
	case URLEncode:
		return []byte(url.QueryEscape(string(input))), nil
	case URLDecode:
		s, err := url.QueryUnescape(string(input))
		if err != nil {
			return nil, fmt.Errorf("url decode: %w", err)
		}
		return []byte(s), nil
	case Base64Encode:
		return []byte(base64.StdEncoding.EncodeToString(input)), nil
	case Base64Decode:
		return decodeBase64Tolerant(string(input))
	case Base64URLEncode:
		return []byte(base64.RawURLEncoding.EncodeToString(input)), nil
	case Base64URLDecode:
		return decodeBase64Tolerant(string(input))
	case HexEncode:
		return []byte(hex.EncodeToString(input)), nil
	case HexDecode:
		b, err := hex.DecodeString(stripSpace(string(input)))
		if err != nil {
			return nil, fmt.Errorf("hex decode: %w", err)
		}
		return b, nil
	case HTMLEncode:
		return []byte(html.EscapeString(string(input))), nil
	case HTMLDecode:
		return []byte(html.UnescapeString(string(input))), nil
	case GzipCompress:
		return gzipCompress(input)
	case GzipDecompress:
		return gzipDecompress(input)
	case JWTDecode:
		return jwtDecode(string(input))
	default:
		return nil, fmt.Errorf("unknown operation %q", op)
	}
}

// Chain applies operations left to right, threading each output into the next.
func Chain(ops []Op, input []byte) ([]byte, error) {
	out := input
	for _, op := range ops {
		var err error
		out, err = Apply(op, out)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}
	return out, nil
}

// Detect makes a best-effort guess at how to decode input one step, returning
// the suggested operation and true, or false if nothing obvious applies.
func Detect(input []byte) (Op, bool) {
	s := strings.TrimSpace(string(input))
	if s == "" {
		return "", false
	}
	if looksLikeJWT(s) {
		return JWTDecode, true
	}
	if strings.Contains(s, "%") {
		if _, err := url.QueryUnescape(s); err == nil {
			return URLDecode, true
		}
	}
	if isHex(s) {
		return HexDecode, true
	}
	if _, err := decodeBase64Tolerant(s); err == nil && isMostlyPrintable(s) {
		return Base64Decode, true
	}
	return "", false
}

func decodeBase64Tolerant(s string) ([]byte, error) {
	s = stripSpace(s)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("not valid base64")
}

func gzipCompress(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(input); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(input []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	return out, nil
}

// jwtDecode decodes a JWT's header and payload (signature is not verified) and
// returns them as indented JSON.
func jwtDecode(s string) ([]byte, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("not a JWT (expected header.payload.signature)")
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jwt header: %w", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt payload: %w", err)
	}
	out := map[string]json.RawMessage{
		"header":  json.RawMessage(header),
		"payload": json.RawMessage(payload),
	}
	pretty, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("jwt: %w", err)
	}
	return pretty, nil
}

func looksLikeJWT(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	return json.Valid(header)
}

func isHex(s string) bool {
	s = stripSpace(s)
	if s == "" || len(s)%2 != 0 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func isMostlyPrintable(s string) bool {
	b, err := decodeBase64Tolerant(s)
	if err != nil || len(b) == 0 {
		return false
	}
	printable := 0
	for _, c := range b {
		if c == '\t' || c == '\n' || c == '\r' || (c >= 0x20 && c < 0x7f) {
			printable++
		}
	}
	return printable*100/len(b) >= 85
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)
}
