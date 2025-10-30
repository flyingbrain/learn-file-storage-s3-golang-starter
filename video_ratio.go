package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

type Stream struct {
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type FFProbeResult struct {
	Streams []Stream `json:"streams"`
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe error: %v, output: %s", err, out.String())
	}

	var result FFProbeResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var height, width int

	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			width = stream.Width
			height = stream.Height
		}
	}

	actual := float64(width) / float64(height)
	name := ""
	sDiff := math.MaxFloat64

	commonRatios := map[string]float64{
		"1:1":  1.0,
		"4:3":  4.0 / 3.0,
		"16:9": 16.0 / 9.0,
		"9:16": 9.0 / 16.0,
		"21:9": 21.0 / 9.0,
		"3:2":  3.0 / 2.0,
		"5:4":  5.0 / 4.0,
		"32:9": 32.0 / 9.0,
	}

	for rName, ratio := range commonRatios {
		diff := math.Abs(ratio - actual)
		if diff < sDiff {
			name = rName
			sDiff = diff
		}
	}

	if name != "16:9" && name != "9:16" {
		name = "other"
	}

	return name, nil
}

func processVideoForFastStart(filepath string) (string, error) {
	outPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %v, output: %s", err, cmd.Stdout)
	}

	return outPath, nil
}
