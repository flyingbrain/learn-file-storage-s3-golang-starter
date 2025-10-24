package main

import (
	"crypto/rand"
	"encoding/base64"
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

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
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

	if mediatype != "image/png" && mediatype != "image/jpeg" {
		respondWithError(w, http.StatusInternalServerError, "allowed only image", nil)
		return
	}

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can not find the video", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permissions", nil)
		return
	}

	filetype := strings.Split(mediatype, "/")
	randKey := make([]byte, 32)
	rand.Read(randKey)
	fname := base64.RawURLEncoding.EncodeToString(randKey)
	name := fmt.Sprintf("%s.%s", fname, filetype[1])
	path := filepath.Join(cfg.assetsRoot, name)

	imgFile, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Not able to create file", err)
	}

	_, err = io.Copy(imgFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Not able to create file", err)
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, name)
	videoData.ThumbnailURL = &url

	if err := cfg.db.UpdateVideo(videoData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
