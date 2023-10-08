package chat

import (
	"context"
	"github.com/mylxsw/aidea-server/internal/ai/baichuan"
)

type BaichuanAIChat struct {
	ai *baichuan.BaichuanAI
}

func NewBaichuanAIChat(ai *baichuan.BaichuanAI) *BaichuanAIChat {
	return &BaichuanAIChat{ai: ai}
}

func (ai *BaichuanAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	//TODO implement me
	panic("implement me")
}

func (ai *BaichuanAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	//TODO implement me
	panic("implement me")
}

func (ai *BaichuanAIChat) MaxContextLength(model string) int {
	// TODO 未找到相关文档记载
	return 4000
}
