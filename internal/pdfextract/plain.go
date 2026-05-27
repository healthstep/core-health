package pdfextract

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	pdf "github.com/ledongthuc/pdf"
)

const MaxPDFBytes = 40 << 20

// PlainTextBest tries pdftotext (poppler, preserves table layout) first,
// falls back to ledongthuc/pdf if not available or if it returns too little text.
func PlainTextBest(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF")
	}
	if len(data) > MaxPDFBytes {
		return "", fmt.Errorf("PDF larger than %d bytes", MaxPDFBytes)
	}

	if text, err := extractWithPdftotext(data); err == nil {
		// pdftotext succeeded and returned meaningful content
		if utf8.RuneCountInString(strings.TrimSpace(text)) > 50 {
			return text, nil
		}
	}

	// Fallback to Go library
	return plainTextGo(data)
}

// PlainText uses the Go library only (kept for backward compat).
func PlainText(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty PDF")
	}
	if len(data) > MaxPDFBytes {
		return "", fmt.Errorf("PDF larger than %d bytes", MaxPDFBytes)
	}
	return plainTextGo(data)
}

func extractWithPdftotext(data []byte) (string, error) {
	pdftotextBin, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found: %w", err)
	}

	tmp, err := os.CreateTemp("", "pdfextract-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	// -layout: preserve the original physical layout of the text
	// -enc UTF-8: force UTF-8 output
	// "-" as output: write to stdout
	cmd := exec.Command(pdftotextBin, "-layout", "-enc", "UTF-8", tmp.Name(), "-")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(out), nil
}

func plainTextGo(data []byte) (string, error) {
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
