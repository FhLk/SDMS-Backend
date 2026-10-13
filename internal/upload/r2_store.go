package upload

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type r2API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// R2Store เก็บไฟล์เป็น object ใน bucket โดยใช้ prefix แทนโฟลเดอร์จริง.
type R2Store struct {
	client r2API
	bucket string
	prefix string
}

func NewR2Store(ctx context.Context, cfg config.R2Config) (*R2Store, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}
	prefix, err := normalizeR2Prefix(cfg.Prefix)
	if err != nil {
		return nil, err
	}

	// R2 รองรับ Signature V4 แต่ไม่ได้รองรับ full-object CRC32 ที่ SDK รุ่นใหม่เปิดเป็นค่าเริ่มต้น
	// จึงคำนวณ checksum เฉพาะ operation ที่บังคับใช้ตามตาราง S3 compatibility ของ R2.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)),
		awsconfig.WithRegion("auto"),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("create Cloudflare R2 configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		// path-style ทำให้ endpoint ของ account ใช้ได้ตรงกับรูปแบบ /bucket/object-key.
		options.UsePathStyle = true
	})
	return &R2Store{client: client, bucket: cfg.Bucket, prefix: prefix}, nil
}

func (s *R2Store) RootID() string { return s.prefix }

func (s *R2Store) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	if err := validatePathSegment(name); err != nil {
		return "", err
	}
	// R2 ไม่มี directory จริง การต่อ prefix จึงไม่สร้าง network request หรือ object เปล่า.
	return strings.TrimPrefix(path.Join("/", parentID, name), "/"), nil
}

func (s *R2Store) Put(ctx context.Context, folderID, name, mimeType string, size int64, content io.Reader) (string, error) {
	if err := validatePathSegment(name); err != nil {
		return "", err
	}
	key := strings.TrimPrefix(path.Join("/", folderID, name), "/")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          content,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(mimeType),
	})
	if err != nil {
		return "", fmt.Errorf("upload object to Cloudflare R2: %w", err)
	}
	return key, nil
}

func (s *R2Store) Trash(ctx context.Context, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("delete object from Cloudflare R2: %w", err)
	}
	return nil
}

func normalizeR2Prefix(value string) (string, error) {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return "", nil
	}
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if err := validatePathSegment(segment); err != nil {
			return "", fmt.Errorf("R2_PREFIX contains an unsafe segment %q: %w", segment, err)
		}
	}
	return strings.Join(segments, "/"), nil
}
