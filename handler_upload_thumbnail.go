package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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

	mediaType := header.Header.Get("Content-Type")

	fileExtension, err := getFileExtensionFromMime(mediaType)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Uploaded file is not an image", err)
		return
	}

	key := make([]byte, 32)
	rand.Read(key)
	fileName := base64.RawURLEncoding.EncodeToString(key) + fileExtension

	filePath := filepath.Join(cfg.assetsRoot, fileName)

	fileOnDisk, err := os.Create(filePath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save a file", err)
		return
	}

	newUrl := fmt.Sprintf("http://localhost:%s/%s", cfg.port, filePath)

	_, err = io.Copy(fileOnDisk, file)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file to disk", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Unable to retrieve video metadata",
			err,
		)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized error", err)
		return
	}

	videoMetadata.ThumbnailURL = &newUrl
	err = cfg.db.UpdateVideo(videoMetadata)

	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Error updating video data",
			err,
		)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetadata)
}

func getFileExtensionFromMime(mimeType string) (string, error) {

	const mimePrefix = "image/"

	if !strings.HasPrefix(mimeType, mimePrefix) {
		return "", errors.New("Given file type is not an image")
	}

	result := strings.Split(mimeType, mimePrefix)

	return "." + result[1], nil

}
