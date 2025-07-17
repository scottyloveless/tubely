package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	buffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	type dimensions struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}
	}

	videoDimensions := dimensions{}

	err = json.Unmarshal(buffer.Bytes(), &videoDimensions)
	if err != nil {
		return "", err
	}

	if len(videoDimensions.Streams) == 0 {
		return "", fmt.Errorf("no stream found")
	}

	heightInt := videoDimensions.Streams[0].Height
	widthInt := videoDimensions.Streams[0].Width
	if widthInt <= 0 || heightInt <= 0 {
		return "", fmt.Errorf("invalid dimensions")
	}

	target16_9 := 16.0 / 9.0
	target9_16 := 9.0 / 16.0
	widthRatio := float64(widthInt) / float64(heightInt)
	epsilon := 0.185

	if math.Abs(widthRatio-target16_9) < epsilon {
		return "16:9", nil
	} else if math.Abs(widthRatio-target9_16) < epsilon {
		return "9:16", nil
	} else {
		return "other", nil
	}
}
