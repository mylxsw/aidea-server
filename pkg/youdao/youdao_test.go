package youdao_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"os"
	"testing"

	"github.com/mylxsw/go-utils/must"
	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	client := youdao.NewClient("https://openapi.youdao.com/api", os.Getenv("YOUDAO_TRANSLATE_APPID"), os.Getenv("YOUDAO_TRANSLATE_APPKEY"))
	{
		res, err := client.Translate(context.TODO(), "As an AI language model, I am not capable of feeling emotions, but I can contextually interpret the widely-used emoticon of \"😍\" to depict admiration, love, or infatuation towards someone or something.As an AI language model, I am not capable of feeling emotions, but I can contextually interpret the widely-used emoticon of \"😍\" to depict admiration, love, or infatuation towards someone or something.", youdao.LanguageAuto, youdao.LanguageEnglish)
		assert.NoError(t, err)
		if err == nil {
			assert.NotEmpty(t, res.Result)
			assert.NotEmpty(t, res.SpeakURL)
			fmt.Println(string(must.Must(json.Marshal(res))))
		}
	}

	{
		res, err := client.Translate(context.TODO(), "大风起兮云飞扬", youdao.LanguageAuto, youdao.LanguageEnglish)
		assert.NoError(t, err)
		if err == nil {
			assert.NotEmpty(t, res.Result)
			assert.NotEmpty(t, res.SpeakURL)
			fmt.Println(string(must.Must(json.Marshal(res))))
		}
	}
}
