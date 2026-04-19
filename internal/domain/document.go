package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type DocumentFormat string

const (
	FormatTXT  DocumentFormat = "txt"
	FormatDOCX DocumentFormat = "docx"
)

func ParseDocumentFormat(name string) (DocumentFormat, bool) {
	switch strings.ToLower(strings.TrimPrefix(name, ".")) {
	case string(FormatTXT):
		return FormatTXT, true
	case string(FormatDOCX):
		return FormatDOCX, true
	default:
		return "", false
	}
}

type Document struct {
	ID         uuid.UUID
	Name       string
	OriginPath string
	Format     DocumentFormat
	RawText    string
	SizeBytes  int64
	CreatedAt  time.Time
}
