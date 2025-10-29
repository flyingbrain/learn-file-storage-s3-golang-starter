package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 30

	r.ParseMultipartForm(maxMemory)

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can not find the video", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permissions", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	t := header.Header.Get("Content-type")

	mediatype, _, err := mime.ParseMediaType(t)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can not get filetype", err)
		return
	}
	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusInternalServerError, "allowed only image", nil)
		return
	}

	filetype := strings.Split(mediatype, "/")
	name := fmt.Sprintf("%s.%s", "tubely-upload", filetype[1])
	tFile, err := os.CreateTemp("./samples", name)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "can not create file", err)
	}

	defer tFile.Close()
	defer os.Remove(tFile.Name())

	_, err = io.Copy(tFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "coppy error", err)
	}

	ratio, err := getVideoAspectRatio(tFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}
	if ratio == "16:9" {
		ratio = "landscape"
	}

	if ratio == "9:16" {
		ratio = "portrait"
	}

	s3FileName := ratio + "/" + hex.EncodeToString(b) + ".mp4"
	tFile.Seek(0, io.SeekStart)
	s3Params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &s3FileName,
		Body:        tFile,
		ContentType: &mediatype,
	}

	_, err = cfg.s3Client.PutObject(context.Background(), &s3Params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "s3 file uploading error", err)
	}

	url := fmt.Sprintf("http://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, s3FileName)
	videoData.VideoURL = &url

	if err := cfg.db.UpdateVideo(videoData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
