package baidu_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestBaiduAI_Chat(t *testing.T) {
	testBaiduAI_Chat(t, baidu.ModelErnieBot4)
}

func TestBaiduAI_ChatStream(t *testing.T) {
	testBaiduAI_ChatStream(t, baidu.ModelErnieBot4)
}

func testBaiduAI_Chat(t *testing.T, model baidu.Model) {
	messages := []baidu.ChatMessage{
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "蓝牙耳机坏了去看牙科还是耳科？",
		},
		{
			Role:    baidu.ChatMessageRoleAssistant,
			Content: "蓝牙耳机就是将蓝牙技术应用在免持耳机上，让使用者可以免除恼人电线的牵绊，自在地以各种方式轻松通话。自从蓝牙耳机问世以来，一直是行动商务族提升效率的好工具。\\n蓝牙耳机坏了应该去修理蓝牙耳机，而不是看牙科医生或耳科医生。",
		},
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "但是牙科医生说要去耳科，怎么办？",
		},
	}

	client := baidu.NewBaiduAI(os.Getenv("BAIDU_WXQF_API_KEY"), os.Getenv("BAIDU_WXQF_SECRET"))
	chatResp, err := client.Chat(
		model,
		baidu.ChatRequest{
			Messages: messages,
		},
	)
	assert.NoError(t, err)

	log.With(chatResp).Debug("chat response")
}

func testBaiduAI_ChatStream(t *testing.T, model baidu.Model) {
	messages := []baidu.ChatMessage{
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "蓝牙耳机坏了去看牙科还是耳科？",
		},
		{
			Role:    baidu.ChatMessageRoleAssistant,
			Content: "蓝牙耳机就是将蓝牙技术应用在免持耳机上，让使用者可以免除恼人电线的牵绊，自在地以各种方式轻松通话。自从蓝牙耳机问世以来，一直是行动商务族提升效率的好工具。\\n蓝牙耳机坏了应该去修理蓝牙耳机，而不是看牙科医生或耳科医生。",
		},
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "但是牙科医生说要去耳科，怎么办？",
		},
	}

	client := baidu.NewBaiduAI(os.Getenv("BAIDU_WXQF_API_KEY"), os.Getenv("BAIDU_WXQF_SECRET"))
	chatResp, err := client.ChatStream(
		model,
		baidu.ChatRequest{
			Messages: messages,
		},
	)
	assert.NoError(t, err)

	tokenConsumed, promptTokens := 0, 0
	for res := range chatResp {
		//if !res.IsEND {
		//	fmt.Print(res.Result)
		//} else {
		//	fmt.Println()
		//}
		fmt.Println("-> " + res.Result)

		tokenConsumed, promptTokens = res.Usage.TotalTokens, res.Usage.PromptTokens
	}

	log.Debugf("token consumed: %d, prompt tokens: %d", tokenConsumed, promptTokens)
}

func TestChatMessageFix(t *testing.T) {
	messages := []baidu.ChatMessage{
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "蓝牙耳机坏了去看牙科还是耳科？",
		},
		{
			Role:    baidu.ChatMessageRoleAssistant,
			Content: "蓝牙耳机就是将蓝牙技术应用在免持耳机上，让使用者可以免除恼人电线的牵绊，自在地以各种方式轻松通话。自从蓝牙耳机问世以来，一直是行动商务族提升效率的好工具。\\n蓝牙耳机坏了应该去修理蓝牙耳机，而不是看牙科医生或耳科医生。",
		},
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "但是牙科医生说要去耳科，怎么办？",
		},
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "但是牙科医生说要去耳科，怎么办？",
		},
		{
			Role:    baidu.ChatMessageRoleUser,
			Content: "但是牙科医生说要去耳科，怎么办？",
		},
		{
			Role:    baidu.ChatMessageRoleAssistant,
			Content: "...",
		},
		{
			Role:    baidu.ChatMessageRoleAssistant,
			Content: "...",
		},
	}

	req := baidu.ChatRequest{Messages: messages}

	for _, msg := range req.Fix(baidu.ModelErnieBot).Messages {
		log.Debugf("%s: %s", msg.Role, msg.Content)
	}
}
