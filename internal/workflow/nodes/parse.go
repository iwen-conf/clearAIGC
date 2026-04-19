package nodes

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(_ context.Context, path string, format domain.DocumentFormat) (string, error) {
	switch format {
	case domain.FormatTXT:
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return normalizeNewlines(string(data)), nil
	case domain.FormatDOCX:
		return parseDOCX(path)
	default:
		return "", fmt.Errorf("unsupported format %s", format)
	}
}

func normalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func parseDOCX(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var document io.ReadCloser
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		document, err = file.Open()
		if err != nil {
			return "", err
		}
		defer document.Close()
		break
	}
	if document == nil {
		return "", fmt.Errorf("word/document.xml was not found")
	}

	decoder := xml.NewDecoder(document)
	var paragraphs []string
	var paragraph bytes.Buffer
	inText := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				text := strings.TrimSpace(paragraph.String())
				paragraphs = append(paragraphs, text)
				paragraph.Reset()
			}
		case xml.CharData:
			if inText {
				paragraph.WriteString(string(t))
			}
		}
	}

	return strings.TrimSpace(strings.Join(paragraphs, "\n\n")), nil
}
