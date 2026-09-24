// Package objectstore stores immutable rendered PNGs in a private S3 bucket.
package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// MaxImage is the maximum accepted encoded image size.
const MaxImage int64 = 16 << 20

// Config supplies a private bucket and explicit credentials, never ambient AWS credentials.
type Config struct {
	Endpoint, Region, Bucket, Prefix string
	AccessKey, SecretKey             string
	Local                            bool
}

// Store is safe for concurrent use. Keys are relative to its configured prefix.
type Store struct {
	client         *s3.Client
	bucket, prefix string
	location       string
}

// FromEnv loads credentials from a protected JSON file.
func FromEnv() (*Store, error) {
	b, err := os.ReadFile(os.Getenv("ART_S3_CREDENTIALS_FILE"))
	if err != nil {
		return nil, errors.New("read ART_S3_CREDENTIALS_FILE")
	}
	var secret struct {
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
	}
	if err := json.Unmarshal(b, &secret); err != nil {
		return nil, errors.New("invalid S3 credentials file")
	}
	return New(Config{Endpoint: os.Getenv("ART_S3_ENDPOINT"), Region: os.Getenv("ART_S3_REGION"), Bucket: os.Getenv("ART_S3_BUCKET"), Prefix: os.Getenv("ART_S3_PREFIX"), AccessKey: secret.AccessKey, SecretKey: secret.SecretKey, Local: os.Getenv("ART_S3_LOCAL") == "true"})
}

// New validates endpoint and prefix; HTTP is restricted to explicitly local development.
func New(c Config) (*Store, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || (u.Scheme != "https" && (!c.Local || u.Scheme != "http")) {
		return nil, errors.New("invalid S3 endpoint; HTTPS required")
	}
	if c.Bucket == "" || c.Region == "" || c.AccessKey == "" || c.SecretKey == "" {
		return nil, errors.New("incomplete S3 configuration")
	}
	if c.Prefix == "" {
		c.Prefix = "artifacts"
	}
	if !safeKey(c.Prefix) {
		return nil, errors.New("invalid S3 prefix")
	}
	client := s3.NewFromConfig(aws.Config{Region: c.Region, Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""), HTTPClient: &http.Client{Timeout: 20 * time.Second}, RetryMaxAttempts: 2, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired}, func(o *s3.Options) { o.BaseEndpoint = aws.String(c.Endpoint); o.UsePathStyle = true })
	return &Store{client: client, bucket: c.Bucket, prefix: c.Prefix + "/", location: c.Endpoint + "/" + c.Bucket + "/" + c.Prefix}, nil
}

// Location binds database cleanup authority to an exact configured bucket prefix.
func (s *Store) Location() string { return s.location }

func safeKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		return false
	}
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("/-_.", c) {
			continue
		}
		return false
	}
	return true
}

// CreateBucket is for explicit bootstrap, never called by ordinary reads or writes.
func (s *Store) CreateBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket})
	var api smithy.APIError
	if errors.As(err, &api) && api.ErrorCode() == "BucketAlreadyOwnedByYou" {
		return nil
	}
	return err
}

// Health checks access without creating objects.
func (s *Store) Health(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	return err
}

// Put writes a complete bounded object. Callers use a unique execution key.
func (s *Store) Put(ctx context.Context, key string, data []byte) error {
	if !safeKey(key) || len(data) == 0 || int64(len(data)) > MaxImage {
		return errors.New("invalid object")
	}
	digest := sha256.Sum256(data)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: aws.String(s.prefix + key), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String("image/png"), Metadata: map[string]string{"sha256": hex.EncodeToString(digest[:])}})
	return err
}

// Get verifies length and application digest before exposing bytes to a caller.
func (s *Store) Get(ctx context.Context, key, digest string, size int64) ([]byte, error) {
	if !safeKey(key) || size < 1 || size > MaxImage {
		return nil, errors.New("invalid object metadata")
	}
	res, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: aws.String(s.prefix + key)})
	if err != nil {
		var api smithy.APIError
		if errors.As(err, &api) && api.ErrorCode() == "NoSuchKey" {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	defer res.Body.Close()
	b := make([]byte, int(size))
	if _, err = io.ReadFull(res.Body, b); err != nil {
		return nil, err
	}
	var extra [1]byte
	if n, readErr := res.Body.Read(extra[:]); n != 0 || readErr != io.EOF {
		return nil, errors.New("unexpected trailing image data")
	}
	sum := sha256.Sum256(b)
	if int64(len(b)) != size || hex.EncodeToString(sum[:]) != digest {
		return nil, errors.New("image integrity mismatch")
	}
	return b, nil
}

// Delete removes exactly one immutable object and is idempotent.
func (s *Store) Delete(ctx context.Context, key string) error {
	if !safeKey(key) {
		return errors.New("invalid object key")
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: aws.String(s.prefix + key)})
	return err
}

// Object identifies a listed object for bounded reconciliation.
type Object struct {
	Key      string
	Modified time.Time
}

// List returns one bounded page; callers never scan an entire bucket in one transaction.
func (s *Store) List(ctx context.Context, cursor string) ([]Object, string, error) {
	in := &s3.ListObjectsV2Input{Bucket: &s.bucket, Prefix: &s.prefix, MaxKeys: aws.Int32(100)}
	if cursor != "" {
		in.ContinuationToken = &cursor
	}
	res, err := s.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, "", fmt.Errorf("list images: %w", err)
	}
	var out []Object
	for _, item := range res.Contents {
		if item.Key != nil && item.LastModified != nil {
			out = append(out, Object{strings.TrimPrefix(*item.Key, s.prefix), *item.LastModified})
		}
	}
	return out, aws.ToString(res.NextContinuationToken), nil
}
