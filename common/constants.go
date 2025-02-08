package common

import "time"

var StartTime = time.Now().Unix() // unit: second
var Version = "v1.10.0"           // this hard coding will be replaced automatically when building, no need to manually change

var DefaultOpenaiModelList = []string{
	"gpt-4o",
	"o1",
	"o3-mini-high",
	"claude-3-5-sonnet",
	"claude-3-5-haiku",
	"gemini-2.0-flash",
	"deep-seek-v3",
	"deep-seek-r1",

	"flux",
	"flux-speed",
	"flux-pro/ultra",
	"ideogram",
	"recraft-v3",
	"dall-e-3",
}

var TextModelList = []string{
	"gpt-4o",
	"o1",
	"o3-mini-high",
	"claude-3-5-sonnet",
	"claude-3-5-haiku",
	"gemini-2.0-flash",
	"deep-seek-v3",
	"deep-seek-r1",
}

var MixtureModelList = []string{
	"gpt-4o",
	"claude-3-5-sonnet",
	"gemini-2.0-flash",
}

var ImageModelList = []string{
	"flux",
	"flux-speed",
	"flux-pro/ultra",
	"ideogram",
	"recraft-v3",
	"dall-e-3",
}
