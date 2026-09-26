package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot parse from file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadGateway, "Couldn't find the Content-Type for thumbnail", nil)
		return
	}

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video metadata with the given user ID", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not allowed to update the video", nil)
		return
	}

	imageType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't parse the media type", err)
		return
	}
	if imageType != "image/jpeg" && imageType != "image/png" {
		respondWithError(w, http.StatusUnauthorized, "The file must be in .jpeg or .png format", err)
		return
	}

	imageExtension := strings.Split(imageType, "/")[1]
	filename := fmt.Sprintf("%s.%s", videoIDString, imageExtension)
	filepath := filepath.Join(cfg.assetsRoot, filename)
	imageFile, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create a new file", err)
		return
	}
	defer imageFile.Close()

	_, err = io.Copy(imageFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy contents to the file", err)
		return
	}
	
	thumbnailUrl := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoIDString, imageExtension)
	metadata.ThumbnailURL = &thumbnailUrl

	if err := cfg.db.UpdateVideo(metadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
