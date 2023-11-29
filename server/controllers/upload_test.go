package controllers_test

import (
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/server/controllers"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
	"time"
)

func TestUploadAuditCallback(t *testing.T) {
	data := `{
	   "code": 0,
	   "desc": "The fop was completed successfully",
	   "id": "z0.01z001cwwyu1m7ck3o00mtsqgq001sm5",
	   "inputBucket": "aicode",
	   "inputKey": "ai-server/24/20231113/ugc29bd6ca3-41e0-5977-dbe4-8952e4583059..jpg",
	   "items":
	   [
	       {
	           "cmd": "image-censor/v2/pulp/politician/app/c3RyZWFtX2FpY29kZV9haS1zZXJ2ZXIvX2ltYWdl",
	           "code": 0,
	           "desc": "The fop was completed successfully",
	           "result":
	           {
	               "disable": true,
	               "result":
	               {
	                   "appid": "_kodo_c3RyZWFtX2FpY29kZV9haS1zZXJ2ZXIvX2ltYWdl",
	                   "code": 200,
	                   "disable": true,
	                   "entry_id": "6550fa0b00018704de92cb9f1ea032ca",
	                   "message": "OK",
	                   "scenes":
	                   {
	                       "politician":
	                       {
	                           "result":
	                           {
	                               "label": "normal"
	                           },
	                           "suggestion": "pass"
	                       },
	                       "pulp":
	                       {
	                           "result":
	                           {
	                               "desc": "色情，色情",
	                               "label": "pulp",
	                               "score": 0.9992126,
	                               "sublabel":
	                               [
	                                   "300000"
	                               ]
	                           },
	                           "suggestion": "block"
	                       }
	                   },
	                   "suggestion": "block"
	               }
	           },
	           "returnOld": 0
	       }
	   ],
	   "pipeline": "1380305402.default.sys",
	   "reqid": "YOIAAACrVspI7JYX"
	}`

	var ret controllers.ImageAuditCallback
	assert.NoError(t, json.Unmarshal([]byte(data), &ret))

	log.With(ret).Debugf("item blocked(%v), labels: %v", ret.IsBlocked(), ret.Labels())

	client := uploader.New(&config.Config{
		StorageAppKey:    os.Getenv("QINIU_ACCESS_KEY"),
		StorageAppSecret: os.Getenv("QINIU_SECRET_KEY"),
		StorageDomain:    "https://ssl.aicode.cc",
		StorageBucket:    "aicode",
	})

	fmt.Println(client.MakePrivateURL(ret.InputKey, time.Second*3600*24))
}
