package pdfextract

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

const MaxPDFBytes = 40 << 20

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
