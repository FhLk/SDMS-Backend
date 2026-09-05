package http

import (
	"context"
	"fmt"
	"io"
	"mime"
	"strconv"
	"strings"

	submissiondomain "sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SubmissionFileService interface {
	Upload(
		ctx context.Context,
		topicUID uuid.UUID,
		submissionUID uuid.UUID,
		input usecase.UploadSubmissionFileInput,
	) (*submissiondomain.SubmissionFile, error)

	FindAll(
		ctx context.Context,
		topicUID uuid.UUID,
		submissionUID uuid.UUID,
	) ([]submissiondomain.SubmissionFile, error)

	FindByID(
		ctx context.Context,
		fileUID uuid.UUID,
	) (*submissiondomain.SubmissionFile, error)

	Open(
		ctx context.Context,
		fileUID uuid.UUID,
	) (*submissiondomain.SubmissionFile, io.ReadCloser, error)

	Delete(
		ctx context.Context,
		fileUID uuid.UUID,
	) error
}

type SubmissionFileHandler struct {
	service SubmissionFileService
}

func NewSubmissionFileHandler(service SubmissionFileService) *SubmissionFileHandler {
	return &SubmissionFileHandler{service: service}
}

func (h *SubmissionFileHandler) Upload(c fiber.Ctx) error {
	topicUID, submissionUID, err := parseTopicAndSubmissionUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	fieldUID, err := uuid.Parse(c.FormValue("field_uid"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "invalid field_uid",
		})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "file is required",
		})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "cannot open uploaded file",
		})
	}
	defer file.Close()

	uploaded, err := h.service.Upload(
		c.Context(),
		topicUID,
		submissionUID,
		usecase.UploadSubmissionFileInput{
			FieldUID:         fieldUID,
			OriginalFilename: fileHeader.Filename,
			ContentType:      fileHeader.Header.Get("Content-Type"),
			Size:             fileHeader.Size,
			Reader:           file,
		},
	)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(newSubmissionFileResponse(*uploaded))
}

func (h *SubmissionFileHandler) FindAll(c fiber.Ctx) error {
	topicUID, submissionUID, err := parseTopicAndSubmissionUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	files, err := h.service.FindAll(c.Context(), topicUID, submissionUID)
	if err != nil {
		return handleError(c, err)
	}

	response := make([]SubmissionFileResponse, 0, len(files))
	for _, file := range files {
		response = append(response, newSubmissionFileResponse(file))
	}
	return c.JSON(response)
}

func (h *SubmissionFileHandler) FindByID(c fiber.Ctx) error {
	fileUID, err := parseFileUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	file, err := h.service.FindByID(c.Context(), fileUID)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(newSubmissionFileResponse(*file))
}

// View streams a file inline so a browser can render images, PDFs and videos.
// A single HTTP byte range is supported so <video> can seek/play progressively.
func (h *SubmissionFileHandler) View(c fiber.Ctx) error {
	fileUID, err := parseFileUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	file, reader, err := h.service.Open(c.Context(), fileUID)
	if err != nil {
		return handleError(c, err)
	}

	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Disposition", inlineContentDisposition(file.OriginalFilename))

	rangeHeader := strings.TrimSpace(c.Get("Range"))
	if rangeHeader == "" {
		c.Set("Content-Length", strconv.FormatInt(file.Size, 10))
		return c.SendStream(reader, int(file.Size))
	}

	start, end, err := parseByteRange(rangeHeader, file.Size)
	if err != nil {
		_ = reader.Close()
		c.Set("Content-Range", fmt.Sprintf("bytes */%d", file.Size))
		return c.SendStatus(fiber.StatusRequestedRangeNotSatisfiable)
	}

	if start > 0 {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil {
			_ = reader.Close()
			return err
		}
	}

	contentLength := end - start + 1
	c.Status(fiber.StatusPartialContent)
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))

	return c.SendStream(readCloser{Reader: io.LimitReader(reader, contentLength), Closer: reader}, int(contentLength))
}

func (h *SubmissionFileHandler) Download(c fiber.Ctx) error {
	fileUID, err := parseFileUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	file, reader, err := h.service.Open(c.Context(), fileUID)
	if err != nil {
		return handleError(c, err)
	}

	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Attachment(file.OriginalFilename)
	return c.SendStream(reader, int(file.Size))
}

func (h *SubmissionFileHandler) Delete(c fiber.Ctx) error {
	fileUID, err := parseFileUID(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	if err := h.service.Delete(c.Context(), fileUID); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type readCloser struct {
	io.Reader
	io.Closer
}

func parseTopicAndSubmissionUID(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	topicUID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid topic id")
	}

	submissionUID, err := uuid.Parse(c.Params("submissionID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid submission id")
	}
	return topicUID, submissionUID, nil
}

func parseFileUID(c fiber.Ctx) (uuid.UUID, error) {
	fileUID, err := uuid.Parse(c.Params("fileID"))
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid file id")
	}
	return fileUID, nil
}

func badRequest(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"message": message,
	})
}

func inlineContentDisposition(filename string) string {
	disposition := mime.FormatMediaType("inline", map[string]string{
		"filename": filename,
	})
	if disposition == "" {
		return "inline"
	}
	return disposition
}

func parseByteRange(value string, size int64) (int64, int64, error) {
	if size <= 0 || !strings.HasPrefix(value, "bytes=") {
		return 0, 0, fmt.Errorf("invalid byte range")
	}

	spec := strings.TrimSpace(strings.TrimPrefix(value, "bytes="))
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, fmt.Errorf("multiple or empty ranges are not supported")
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid byte range")
	}

	startText := strings.TrimSpace(parts[0])
	endText := strings.TrimSpace(parts[1])

	if startText == "" {
		suffixLength, err := strconv.ParseInt(endText, 10, 64)
		if err != nil || suffixLength <= 0 {
			return 0, 0, fmt.Errorf("invalid suffix range")
		}
		if suffixLength > size {
			suffixLength = size
		}
		return size - suffixLength, size - 1, nil
	}

	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, fmt.Errorf("invalid range start")
	}

	if endText == "" {
		return start, size - 1, nil
	}

	end, err := strconv.ParseInt(endText, 10, 64)
	if err != nil || end < start {
		return 0, 0, fmt.Errorf("invalid range end")
	}
	if end >= size {
		end = size - 1
	}

	return start, end, nil
}
