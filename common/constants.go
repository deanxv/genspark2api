package common

import "time"

var StartTime = time.Now().Unix() // unit: second

var Version = "v1.12.6" // this hard coding will be replaced automatically when building, no need to manually change

var DefaultOpenaiModelList = []string{
	"gpt-5.1-minimal",
	"gpt-5.1",
	"gpt-5.1-high",
	"gpt-5-pro",
	"o3",
	"o3-pro",
	"claude-3-7-sonnet-thinking",
	"claude-3-7-sonnet",
	"claude-sonnet-4-thinking",
	"claude-sonnet-4",
	"claude-sonnet-4-5",
	"claude-opus-4",
	"claude-opus-4-1",
	"claude-4-5-haiku",
	"gemini-2.5-flash",
	"gemini-2.5-pro",
	"gemini-3-pro-preview",
	"kimi-k2-instruct",
	"groq-kimi-k2-instruct",
	"grok-4-0709",
	"fal-ai/nano-banana",
	"fal-ai/bytedance/seedream/v4",
	"gpt-image-1",
	"flux-pro/ultra",
	"flux-pro/kontext/pro",
	"imagen4",
	"sora-2",
	"sora-2-pro",
	"gemini/veo3",
	"gemini/veo3/fast",
	"kling/v2.5-turbo/pro",
	"fal-ai/bytedance/seedance/v1/pro",
	"minimax/hailuo-02/standard",
	"pixverse/v5",
	"fal-ai/bytedance/seedance/v1/lite",
	"gemini/veo2",
	"wan/v2.2",
	"hunyuan",
	"vidu/start-end-to-video",
	"runway/gen4_turbo",
}

var TextModelList = []string{
	"gpt-5.1-minimal",
	"gpt-5.1",
	"gpt-5.1-high",
	"gpt-5-pro",
	"o3",
	"o3-pro",
	"claude-3-7-sonnet-thinking",
	"claude-3-7-sonnet",
	"claude-sonnet-4-thinking",
	"claude-sonnet-4",
	"claude-sonnet-4-5",
	"claude-opus-4",
	"claude-opus-4-1",
	"claude-4-5-haiku",
	"gemini-2.5-flash",
	"gemini-2.5-pro",
	"gemini-3-pro-preview",
	"kimi-k2-instruct",
	"groq-kimi-k2-instruct",
	"grok-4-0709",
}

var MixtureModelList = []string{
	"gpt-5",
	"claude-sonnet-4",
	"gemini-2.5-flash",
}

var ImageModelList = []string{
	"fal-ai/nano-banana",
	"fal-ai/bytedance/seedream/v4",
	"gpt-image-1",
	"flux-pro/ultra",
	"flux-pro/kontext/pro",
	"imagen4",
}

var VideoModelList = []string{
	"sora-2",
	"sora-2-pro",
	"gemini/veo3",
	"gemini/veo3/fast",
	"kling/v2.5-turbo/pro",
	"fal-ai/bytedance/seedance/v1/pro",
	"minimax/hailuo-02/standard",
	"pixverse/v5",
	"fal-ai/bytedance/seedance/v1/lite",
	"gemini/veo2",
	"wan/v2.2",
	"hunyuan",
	"vidu/start-end-to-video",
	"runway/gen4_turbo",
}

//
