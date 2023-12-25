package stabilityai

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

type VideoError struct {
	Name   string   `json:"name,omitempty"`
	Errors []string `json:"errors,omitempty"`
}
