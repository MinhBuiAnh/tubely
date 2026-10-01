package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type VideoMetadata struct {
	Streams []struct {
		Index int `json:"index"`
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
		Width int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

const tolerance = 0.01

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer

	if err := cmd.Run(); err != nil {
		return "", err
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(buffer.Bytes(), &metadata); err != nil {
		return "", err
	}

	for _, stream := range metadata.Streams {
		if stream.CodecType == "video" {
			if stream.Width > 0 && stream.Height > 0 {
				aspectRatio := float64(stream.Width) / float64(stream.Height)
				
				if aspectRatio > 1.77-tolerance && aspectRatio < 1.77+tolerance {
					return "16:9", nil
				} else if aspectRatio > 0.56-tolerance && aspectRatio < 0.56+tolerance {
					return "9:16", nil
				} else {
					return "other", nil
				}
			}
		}
	}

	return "", fmt.Errorf("no video stream found")
}