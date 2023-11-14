package baidu

import "context"

type FakeBaiduAI struct{}

func (f FakeBaiduAI) Chat(ctx context.Context, model Model, req ChatRequest) (*ChatResponse, error) {
	return nil, nil
}

func (f FakeBaiduAI) ChatStream(ctx context.Context, model Model, req ChatRequest) (<-chan ChatResponse, error) {
	return nil, nil
}
