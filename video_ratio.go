package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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

func gneratePresignedURL(s3Client *s3.Client, bucket, key string, expiteTime time.Duration) (string, error) {
	pClint := s3.NewPresignClient(s3Client)
	req, err := pClint.PresignGetObject(context.Background(),
		&s3.GetObjectInput{Bucket: &bucket, Key: &key},
		s3.WithPresignExpires(expiteTime))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	params := strings.Split(*video.VideoURL, ",")
	if len(params) != 2 {
		return video, fmt.Errorf("Incorect video URL")
	}
	url, err := gneratePresignedURL(cfg.s3Client, params[0], params[1], time.Minute)
	if err != nil {
		return video, err
	}

	video.VideoURL = &url

	return video, nil
}
