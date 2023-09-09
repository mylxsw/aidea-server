package youdao

import "context"

type FakeTranslater struct {
}

func (f *FakeTranslater) TranslateToEnglish(text string) string {
	return text
}

func (f *FakeTranslater) Translate(ctx context.Context, from, target string, text string) (*TranslateResult, error) {
	return &TranslateResult{Result: text}, nil
}
