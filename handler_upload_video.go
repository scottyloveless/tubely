package main

import (
	"crypto/rand"
	"encoding/base64"
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
	const maxUploadSize = 1 << 30 // 1 GB

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	videoID := r.PathValue("videoID")

	uuidParse, err := uuid.Parse(videoID)
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

	dbVideo, err := cfg.db.GetVideo(uuidParse)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "cannot edit someone else's video", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error getting file and header", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusInternalServerError, "please upload mp4 video", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating temporary file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close() // LIFO

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying to temp file", err)
		return
	}

	tempFile.Seek(0, io.SeekStart)

	aspRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error with video dimensions", err)
		return
	}
	var aspPrefix string

	switch aspRatio {
	case "16:9":
		aspPrefix = "landscape"
	case "9:16":
		aspPrefix = "portrait"
	case "other":
		aspPrefix = "other"
	}

	fastVideoName, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error moving metadata to front", err)
		return
	}

	fastVideoFile, err := os.Open(fastVideoName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error opening fast video file", err)
		return
	}
	defer os.Remove(fastVideoName)
	defer fastVideoFile.Close()

	byteSlice := make([]byte, 32)
	rand.Read(byteSlice)
	encoding := base64.RawURLEncoding.EncodeToString(byteSlice)
	fileNameString := "/" + aspPrefix + "/" + encoding + ".mp4"
	contentTypeString := "video/mp4"

	params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileNameString,
		Body:        fastVideoFile,
		ContentType: &contentTypeString,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error adding object to cdn", err)
		return
	}

	newUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fileNameString)

	dbVideo.VideoURL = &newUrl

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
