package main

import (
	"bytes"
	"os/exec"
)

func processVideoForFastStart(filePath string) (string, error) {
	outPutFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPutFilePath)

	buffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outPutFilePath, nil
}
