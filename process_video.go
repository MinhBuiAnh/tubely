package main

import "os/exec"

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath[:len(filePath) - 4] + ".processing" + ".mp4"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputFilePath, nil
}