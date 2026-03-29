package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const UploadDir = "./uploads"

// SaveFile saves an uploaded file and returns the relative path
func SaveFile(c *fiber.Ctx, fieldName string, subDir string) (string, error) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	// Create directory if not exists
	dir := filepath.Join(UploadDir, subDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), strings.TrimSuffix(file.Filename, ext), ext)
	filePath := filepath.Join(dir, filename)

	// Save file
	if err := c.SaveFile(file, filePath); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return relative path (for URL)
	return "/" + filepath.ToSlash(filePath), nil
}
