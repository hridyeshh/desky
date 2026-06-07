package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// maxUploadBytes caps GIF uploads at 10 MB.
const maxUploadBytes = 10 << 20

// cloudinaryUploadResponse is the subset of Cloudinary's upload reply we use.
type cloudinaryUploadResponse struct {
	SecureURL string `json:"secure_url"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// UploadGIF accepts a multipart form file field "file", forwards the bytes to
// Cloudinary's unsigned upload endpoint, and returns {"url": secure_url}.
//
// Requires env:
//   CLOUDINARY_CLOUD_NAME    — your Cloudinary cloud name
//   CLOUDINARY_UPLOAD_PRESET — an unsigned upload preset
func (h *Handlers) UploadGIF(w http.ResponseWriter, r *http.Request) {
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	preset := os.Getenv("CLOUDINARY_UPLOAD_PRESET")
	if cloudName == "" || preset == "" {
		writeError(w, http.StatusServiceUnavailable,
			"Cloudinary not configured (set CLOUDINARY_CLOUD_NAME and CLOUDINARY_UPLOAD_PRESET)")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or malformed multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, `missing form field "file"`)
		return
	}
	defer file.Close()

	// Validate GIF magic bytes (GIF87a / GIF89a).
	head := make([]byte, 6)
	if _, err := io.ReadFull(file, head); err != nil {
		writeError(w, http.StatusBadRequest, "could not read file")
		return
	}
	if string(head) != "GIF87a" && string(head) != "GIF89a" {
		writeError(w, http.StatusBadRequest, "file is not a GIF")
		return
	}

	// Build the multipart body for Cloudinary, re-prepending the magic bytes.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("upload_preset", preset); err != nil {
		writeError(w, http.StatusInternalServerError, "could not build upload")
		return
	}
	part, err := mw.CreateFormFile("file", header.Filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build upload")
		return
	}
	if _, err := part.Write(head); err != nil {
		writeError(w, http.StatusInternalServerError, "could not build upload")
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not read upload")
		return
	}
	mw.Close()

	endpoint := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", cloudName)
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create upload request")
		return
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Cloudinary unreachable")
		return
	}
	defer resp.Body.Close()

	var cl cloudinaryUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&cl); err != nil {
		writeError(w, http.StatusBadGateway, "could not parse Cloudinary response")
		return
	}
	if cl.Error != nil {
		writeError(w, http.StatusBadGateway, "Cloudinary: "+cl.Error.Message)
		return
	}
	if cl.SecureURL == "" {
		writeError(w, http.StatusBadGateway, "Cloudinary returned no URL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": cl.SecureURL})
}
