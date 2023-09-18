package baidu

type FakeBaiduAI struct{}

func (f FakeBaiduAI) Chat(model Model, req ChatRequest) (*ChatResponse, error) {
	return nil, nil
}

func (f FakeBaiduAI) ChatStream(model Model, req ChatRequest) (<-chan ChatResponse, error) {
	return nil, nil
}
