package pdfextract

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// MaxPDFBytes limits streamed upload size (single concatenated file).
const MaxPDFBytes = 40 << 20 // 40 MiB

// PlainText extracts human-readable text from a PDF byte slice (embedded fonts / UTF-16 / ToUnicode).
// Pure Go; suitable for CGO_ENABLED=0 builds. Complex layouts may be imperfect.
func PlainText(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF")
	}
	if len(data) > MaxPDFBytes {
		return "", fmt.Errorf("PDF larger than %d bytes", MaxPDFBytes)
	}
	r := bytes.NewReader(data)
	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	rc, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("plain text: %w", err)
	}
	var sb strings.Builder
	if _, err := io.Copy(&sb, rc); err != nil {
		return "", fmt.Errorf("read plain text: %w", err)
	}
	return sb.String(), nil
}
