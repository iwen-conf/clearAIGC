package nodes

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestBuildManifestChinese(t *testing.T) {
	chunker := NewChunker()
	text := "第一段包含比较长的说明文本，需要被切成多个片段，但是不能打乱原有的段落顺序和表达重点。\n\n第二段保持简短。"

	manifest := chunker.BuildManifest(uuid.New(), text, domain.ChunkMetricChar, 20)
	if manifest.ParagraphCount != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", manifest.ParagraphCount)
	}
	if manifest.ChunkCount < 3 {
		t.Fatalf("expected multiple chunks, got %d", manifest.ChunkCount)
	}
	for _, chunk := range manifest.Chunks {
		if chunk.MetricValue > 20 {
			t.Fatalf("chunk %s exceeded limit: %d", chunk.ID, chunk.MetricValue)
		}
	}
}

func TestBuildManifestEnglish(t *testing.T) {
	chunker := NewChunker()
	text := "This is a longer paragraph that should be divided into several smaller parts without losing the paragraph order or the original meaning."

	manifest := chunker.BuildManifest(uuid.New(), text, domain.ChunkMetricWord, 8)
	if manifest.ChunkCount < 2 {
		t.Fatalf("expected more than one chunk, got %d", manifest.ChunkCount)
	}
	for _, chunk := range manifest.Chunks {
		if words := len(strings.Fields(chunk.Text)); words > 8 {
			t.Fatalf("chunk %s exceeded word limit: %d", chunk.ID, words)
		}
	}
}
