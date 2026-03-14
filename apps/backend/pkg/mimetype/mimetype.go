// Package mimetype detects file MIME types from their content using magic bytes.
// It satisfies the github.com/gabriel-vasile/mimetype API used by go-playground/validator.
package mimetype

import (
	"bytes"
	"io"
)

// MIME holds a detected MIME type string.
type MIME struct {
	mime string
}

// String returns the MIME type, e.g. "image/png".
func (m *MIME) String() string {
	if m == nil {
		return "application/octet-stream"
	}
	return m.mime
}

// DetectReader reads up to 512 bytes from r and returns the detected MIME type.
// It never returns an error; on read failure it defaults to application/octet-stream.
func DetectReader(r io.Reader) (*MIME, error) {
	header := make([]byte, 512)
	n, _ := io.ReadFull(r, header)
	return &MIME{mime: detect(header[:n])}, nil
}

// detect returns the MIME type for the given header bytes using magic number matching.
func detect(h []byte) string {
	n := len(h)
	if n == 0 {
		return "application/octet-stream"
	}

	// ── Images ──────────────────────────────────────────────────────────────
	if n >= 3 && h[0] == 0xFF && h[1] == 0xD8 && h[2] == 0xFF {
		return "image/jpeg"
	}
	if n >= 8 && bytes.Equal(h[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if n >= 6 && (bytes.HasPrefix(h, []byte("GIF87a")) || bytes.HasPrefix(h, []byte("GIF89a"))) {
		return "image/gif"
	}
	if n >= 12 && bytes.HasPrefix(h, []byte("RIFF")) && bytes.Equal(h[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	if n >= 2 && h[0] == 0x42 && h[1] == 0x4D {
		return "image/bmp"
	}
	if n >= 4 && (bytes.HasPrefix(h, []byte{0x49, 0x49, 0x2A, 0x00}) ||
		bytes.HasPrefix(h, []byte{0x4D, 0x4D, 0x00, 0x2A})) {
		return "image/tiff"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte{0x00, 0x00, 0x01, 0x00}) {
		return "image/x-icon"
	}
	// AVIF / HEIF / HEIC share the ftyp box structure (check before generic MP4)
	if n >= 12 && bytes.Equal(h[4:8], []byte("ftyp")) {
		brand := string(h[8:12])
		switch brand {
		case "avif", "avis":
			return "image/avif"
		case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
			return "image/heic"
		}
	}

	// ── Audio ────────────────────────────────────────────────────────────────
	// MP3: ID3 tag or raw frame sync
	if n >= 3 && bytes.HasPrefix(h, []byte("ID3")) {
		return "audio/mpeg"
	}
	if n >= 2 && h[0] == 0xFF && (h[1]&0xE0) == 0xE0 {
		return "audio/mpeg"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte("fLaC")) {
		return "audio/flac"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte("OggS")) {
		return "audio/ogg"
	}
	if n >= 12 && bytes.HasPrefix(h, []byte("RIFF")) && bytes.Equal(h[8:12], []byte("WAVE")) {
		return "audio/wave"
	}
	if n >= 8 && bytes.Equal(h[:8], []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11}) {
		return "audio/x-ms-wma"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte("M4A ")) {
		return "audio/mp4"
	}
	// AAC ADTS frame sync
	if n >= 2 && h[0] == 0xFF && (h[1]&0xF0) == 0xF0 {
		return "audio/aac"
	}

	// ── Video ────────────────────────────────────────────────────────────────
	if n >= 4 && bytes.HasPrefix(h, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "video/webm"
	}
	// Generic MP4 / M4V (ftyp box already handled above for AVIF/HEIF)
	if n >= 8 && bytes.Equal(h[4:8], []byte("ftyp")) {
		return "video/mp4"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte{0x00, 0x00, 0x01, 0xBA}) {
		return "video/mpeg"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte{0x00, 0x00, 0x01, 0xB3}) {
		return "video/mpeg"
	}
	if n >= 4 && bytes.HasPrefix(h, []byte("RIFF")) && n >= 12 && bytes.Equal(h[8:12], []byte("AVI ")) {
		return "video/avi"
	}
	if n >= 4 && (bytes.HasPrefix(h, []byte{0x30, 0x26, 0xB2, 0x75})) {
		return "video/x-ms-wmv"
	}

	// ── Documents ────────────────────────────────────────────────────────────
	if n >= 4 && bytes.HasPrefix(h, []byte("%PDF")) {
		return "application/pdf"
	}
	// ZIP (also DOCX, XLSX, PPTX, etc.)
	if n >= 4 && bytes.HasPrefix(h, []byte{0x50, 0x4B, 0x03, 0x04}) {
		// Peek inside for Office Open XML signatures
		if bytes.Contains(h, []byte("word/")) {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
		if bytes.Contains(h, []byte("xl/")) {
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		if bytes.Contains(h, []byte("ppt/")) {
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		}
		return "application/zip"
	}
	if n >= 8 && bytes.HasPrefix(h, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return "application/msword" // legacy OLE2 container (DOC/XLS/PPT)
	}
	// RAR
	if n >= 7 && bytes.HasPrefix(h, []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00}) {
		return "application/x-rar-compressed"
	}
	// 7-Zip
	if n >= 6 && bytes.HasPrefix(h, []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}) {
		return "application/x-7z-compressed"
	}
	// GZip
	if n >= 2 && h[0] == 0x1F && h[1] == 0x8B {
		return "application/gzip"
	}
	// Bzip2
	if n >= 2 && h[0] == 0x42 && h[1] == 0x5A {
		return "application/x-bzip2"
	}
	// Sqlite
	if n >= 16 && bytes.HasPrefix(h, []byte("SQLite format 3\000")) {
		return "application/x-sqlite3"
	}

	// ── Text / markup ────────────────────────────────────────────────────────
	trimmed := bytes.TrimLeft(h, " \t\r\n")
	if bytes.HasPrefix(trimmed, []byte("<!DOCTYPE html")) ||
		bytes.HasPrefix(trimmed, []byte("<html")) ||
		bytes.HasPrefix(trimmed, []byte("<HTML")) {
		return "text/html"
	}
	if bytes.HasPrefix(trimmed, []byte("<?xml")) {
		return "text/xml"
	}
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		if isPrintable(h) {
			return "application/json"
		}
	}
	if isPrintable(h) {
		return "text/plain"
	}

	return "application/octet-stream"
}

// isPrintable reports whether the bytes look like plain text (all bytes are
// printable ASCII or common UTF-8 control chars).
func isPrintable(b []byte) bool {
	for _, c := range b {
		if c < 0x09 || (c > 0x0D && c < 0x20) || c == 0x7F {
			return false
		}
	}
	return true
}
