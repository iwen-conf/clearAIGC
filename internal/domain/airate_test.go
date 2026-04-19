package domain

import "testing"

func TestEstimateAIRateDetectsChineseTemplateWriting(t *testing.T) {
	t.Parallel()

	text := "在当前数字化转型不断深入推进的大背景下，企业知识管理工作正在呈现出快速发展、持续演进以及多元融合的总体趋势。与此同时，越来越多的团队开始认识到，仅仅依靠传统的文档整理方式已经较难满足高频协作、实时更新以及跨部门共享的现实需求。综合来看，这一方向具有现实意义和实践价值。"
	if got := EstimateAIRate(text); got < 0.5 {
		t.Fatalf("expected high aiRate, got %.2f", got)
	}
}

func TestEstimateAIRateKeepsPlainHumanTextLow(t *testing.T) {
	t.Parallel()

	text := "昨晚我把方案又改了一遍，删掉了几段空话，只保留真正需要的结论。今天再看，行文顺了很多。"
	if got := EstimateAIRate(text); got > 0.25 {
		t.Fatalf("expected low aiRate, got %.2f", got)
	}
}

func TestEstimateAIRateDetectsEnglishTemplateWriting(t *testing.T) {
	t.Parallel()

	text := "In the context of digital collaboration, it is worth noting that teams often rely on generic documentation practices. In conclusion, this challenge has practical value and deserves further exploration."
	if got := EstimateAIRate(text); got < 0.35 {
		t.Fatalf("expected elevated aiRate, got %.2f", got)
	}
}
