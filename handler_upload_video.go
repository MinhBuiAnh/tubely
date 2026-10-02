package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video ", videoID, "by user", userID)

	r.Body = http.MaxBytesReader(w, r.Body, 1 << 30)

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video metadata with the given user ID", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not allowed to update the video", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot parse from file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadGateway, "Couldn't find the Content-Type for video", nil)
		return
	}
	videoType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't parse the media type", err)
		return
	}
	if videoType != "video/mp4" {
		respondWithError(w, http.StatusUnauthorized, "The file must be in .mp4 format", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temporary file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy the file to temporary file", err)
		return
	}

	tempFile.Seek(0, io.SeekStart)

	outputFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process the video for fast start", err)
		return
	}
	outputFile, err := os.Open(outputFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open the processed video file", err)
		return
	}
	defer outputFile.Close()
	defer os.Remove(outputFilePath)

	aspectRatio, err := getVideoAspectRatio(outputFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get the aspect ratio of the video", err)
		return
	}

	var prefix string
	switch aspectRatio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)
	randomString := hex.EncodeToString(randomBytes)
	fileKey := fmt.Sprintf("%s/%s.mp4", prefix, randomString)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        outputFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload the file to S3", err)
		return
	}

	updatedVideoUrl := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, fileKey)
	metadata.VideoURL = &updatedVideoUrl
	if err := cfg.db.UpdateVideo(metadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video metadata with the new URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
