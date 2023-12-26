package stabilityai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/must"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
)

type VideoRequest struct {
	// ImagePath The source image used in the video generation process. Please ensure that the source image is in the correct format and dimensions.
	// Supported Formats:
	//   - image/jpeg
	//   - image/png
	// Supported Dimensions:
	//   - 1024x576
	//   - 576x1024
	//   - 768x768
	ImagePath string `json:"image_path,omitempty"`
	// Seed A specific value that is used to guide the 'randomness' of the generation.
	// (Omit this parameter or pass 0 to use a random seed.)
	// number [ 0 .. 2147483648 ], default 0
	Seed int `json:"seed,omitempty"`
	// CfgScale How strongly the video sticks to the original image.
	// Use lower values to allow the model more freedom to make changes and higher values to correct motion distortions.
	// number [ 0 .. 10 ], default 2.5
	CfgScale int `json:"cfg_scale,omitempty"`
	// MotionBucketID Lower values generally result in less motion in the output video,
	// while higher values generally result in more motion.
	// This parameter corresponds to the motion_bucket_id parameter from the paper.
	// number [ 1 .. 255 ], default 40
	MotionBucketID int `json:"motion_bucket_id,omitempty"`
}

type VideoTaskResponse struct {
	ID string `json:"id,omitempty"`
}

type VideoResponse struct {
	// 200

	// Video The generated video.
	Video        string `json:"video,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	Seed         int    `json:"seed,omitempty"`

	// 202
	ID string `json:"id,omitempty"`
	// Status: in-progress
	Status string `json:"status,omitempty"`
}

func (res *VideoResponse) SaveToLocalFiles(ctx context.Context, savePath string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(res.Video)
	if err != nil {
		return "", fmt.Errorf("decode base64 failed: %w", err)
	}

	key := filepath.Join(savePath, fmt.Sprintf("%s.%s", must.Must(uuid.GenerateUUID()), "mp4"))
	if err := os.WriteFile(key, data, os.ModePerm); err != nil {
		return "", fmt.Errorf("write image to file failed: %w", err)
	}

	return key, nil
}

func (res *VideoResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) (string, error) {
	data, err := base64.StdEncoding.DecodeString(res.Video)
	if err != nil {
		return "", fmt.Errorf("decode base64 failed: %w", err)
	}

	ret, err := up.UploadStream(ctx, int(uid), uploader.DefaultUploadExpireAfterDays, data, "mp4")
	if err != nil {
		return "", fmt.Errorf("upload image to qiniu failed: %w", err)
	}

	return ret, nil
}

type VideoError struct {
	Name   string   `json:"name,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

// ImageToVideo Generate a video from an image.
// https://platform.stability.ai/docs/api-reference#tag/v2alphageneration
func (ai *StabilityAI) ImageToVideo(ctx context.Context, imageToVideoReq VideoRequest) (*VideoTaskResponse, error) {
	data := &bytes.Buffer{}
	writer := multipart.NewWriter(data)

	// Write the init image to the request
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "image", "image.png"))
	h.Set("Content-Type", "image/png")

	imageWriter, _ := writer.CreatePart(h)
	imageFile, imageErr := os.Open(imageToVideoReq.ImagePath)
	if imageErr != nil {
		_ = writer.Close()
		return nil, imageErr
	}

	_, _ = io.Copy(imageWriter, imageFile)

	if imageToVideoReq.Seed > 0 {
		_ = writer.WriteField("seed", strconv.Itoa(imageToVideoReq.Seed))
	}

	if imageToVideoReq.CfgScale > 0 {
		_ = writer.WriteField("cfg_scale", strconv.Itoa(imageToVideoReq.CfgScale))
	}

	if imageToVideoReq.MotionBucketID > 0 {
		_ = writer.WriteField("motion_bucket_id", strconv.Itoa(imageToVideoReq.MotionBucketID))
	}

	_ = writer.Close()

	// Execute the request
	payload := bytes.NewReader(data.Bytes())
	req, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/v2alpha/generation/image-to-video", ai.conf.StabilityAIServer[0]), payload)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		req.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	resp, err := ai.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody := must.Must(io.ReadAll(resp.Body))
		log.F(log.M{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"body":        string(respBody),
		}).Errorf("failed to decode response body: %v", err)

		return nil, fmt.Errorf("请求失败: %v", string(respBody))
	}

	var body VideoTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

// ImageToVideoResult Get the result of a video generation task.
func (ai *StabilityAI) ImageToVideoResult(ctx context.Context, taskID string) (*VideoResponse, error) {
	// Build the request
	req, _ := http.NewRequestWithContext(ctx, "GET", ai.conf.StabilityAIServer[0]+"/v2alpha/generation/image-to-video/result/"+taskID, nil)
	req.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		req.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	req.Header.Add("Accept", "application/json")

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("请求失败：%v", string(must.Must(io.ReadAll(resp.Body))))
	}

	var ret VideoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &ret, nil
}
