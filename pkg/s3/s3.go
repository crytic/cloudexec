package s3

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/crytic/cloudexec/pkg/config"
)

/*
 * The bucket hub, everything related to s3-style buckets of files
 * exports the following functions:
 * - ListBuckets(config config.Config) ([]string, error)
 * - CreateBucket(config config.Config) error
 * - PutObject(config config.Config, key string, value []byte) error
 * - GetObject(config config.Config, key string) ([]byte, error)
 * - ListObjects(config config.Config, prefix string) ([]string, error)
 * - DeleteObject(config config.Config, key string) error
 */

var s3Client *s3.S3 // cache

// Note: not safe to use concurrently from multiple goroutines (yet?)
func initializeS3Client(config config.Config, init bool) (*s3.S3, error) {
	// Immediately return our cached client if available
	if !init && s3Client != nil {
		return s3Client, nil
	}
	// Unpack required config values
	spacesAccessKey := config.DigitalOcean.SpacesAccessKey
	spacesSecretKey := config.DigitalOcean.SpacesSecretKey
	endpoint := fmt.Sprintf("https://%s.digitaloceanspaces.com", config.DigitalOcean.SpacesRegion)
	// Region must be "us-east-1" when creating new Spaces. Otherwise, use the region in your endpoint, such as "nyc3".
	var spacesRegion string
	if init {
		spacesRegion = "us-east-1"
	} else {
		spacesRegion = config.DigitalOcean.SpacesRegion
	}
	// Configure the Spaces client
	spacesConfig := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(spacesAccessKey, spacesSecretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(spacesRegion),
		S3ForcePathStyle: aws.Bool(false),
	}
	// Create a new session and S3 client
	newSession, err := session.NewSession(spacesConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create S3 client: %w", err)
	}
	if init {
		return s3.New(newSession), nil
	}
	// Cache client for subsequent usage iff not the client used for initialization
	s3Client = s3.New(newSession)
	return s3Client, nil
}

func ListBuckets(config config.Config) ([]string, error) {
	var buckets []string = nil
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return buckets, err
	}
	// get bucket details from s3 provider
	listBucketsOutput, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return buckets, fmt.Errorf("Failed to list buckets: %w", err)
	}
	// extract just the names from bucket details
	for _, bucket := range listBucketsOutput.Buckets {
		buckets = append(buckets, *bucket.Name)
	}
	return buckets, nil
}

func SetVersioning(config config.Config) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	// create a non-init client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return err
	}
	// ensure versioning is enabled on the bucket
	_, err = s3Client.PutBucketVersioning(&s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &s3.VersioningConfiguration{
			Status: aws.String("Enabled"),
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to enable versioning on bucket '%s': %w", bucketName, err)
	}
	return nil
}

func CreateBucket(config config.Config) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	// create an initialization client
	s3Client, err := initializeS3Client(config, true)
	if err != nil {
		return err
	}
	// execution bucket creation
	_, err = s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("Failed to create bucket '%s': %w", bucketName, err)
	}
	// wait for the bucket to be available
	err = s3Client.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("Failed to wait for bucket '%s': %w", bucketName, err)
	}
	return nil
}

// Note: will overwrite existing objects if they already exist
func PutObject(config config.Config, key string, value []byte) error {
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return err
	}
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	// If zero-length value is given, create a directory instead of a file
	if len(value) == 0 {
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("Failed to create %s directory in bucket %s: %w", key, bucketName, err)
		}
		return nil
	}
	// hash the input to ensure the integrity of file
	// Does the s3 sdk do this for us automatically?
	md5Hash := md5.Sum(value)
	md5HashBase64 := base64.StdEncoding.EncodeToString(md5Hash[:])
	// Upload the file
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(bytes.NewReader(value)),
		ACL:         aws.String("private"),
		ContentType: aws.String(http.DetectContentType(value)),
		ContentMD5:  aws.String(md5HashBase64),
	})
	if err != nil {
		return fmt.Errorf("Failed to upload file %s to bucket %s: %w", key, bucketName, err)
	}
	return nil
}

func GetObject(config config.Config, key string) ([]byte, error) {
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return []byte{}, err
	}
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	const maxRetries = 3
	for i := 1; i <= maxRetries; i++ {
		resp, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				// Process AWS S3 error
				switch awsErr.Code() {
				case s3.ErrCodeNoSuchKey:
					return []byte{}, fmt.Errorf("The specified key does not exist.")
				default:
					return []byte{}, fmt.Errorf(err.Error())
				}
			}
			return nil, fmt.Errorf("Failed to get object: %w", err)
		}
		defer resp.Body.Close()
		// Read the object data
		object, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to read object data: %w", err)
		}
		// Calculate the MD5 hash of the downloaded data
		md5Hash := md5.Sum(object)
		md5HashHex := fmt.Sprintf(`"%x"`, md5Hash) // ETag is enclosed in double quotes
		// Compare the calculated MD5 hash with the ETag value
		// Note: may break for large files that are split up amongst several objects
		if resp.ETag == nil || *resp.ETag != md5HashHex {
			if i < maxRetries {
				time.Sleep(time.Duration(i) * time.Second)
				continue
			} else {
				return nil, fmt.Errorf("Data integrity check failed after %d retries: calculated MD5 %s does not match ETag %s", maxRetries, md5HashHex, *resp.ETag)
			}
		}
		return object, nil
	}
	return nil, fmt.Errorf("Failed to get from Spaces bucket: maximum number of retries exceeded")
}

func ListObjects(config config.Config, prefix string) ([]string, error) {
	var objects []string
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return objects, err
	}
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	listObjectsInput := &s3.ListObjectsInput{
		Bucket:  aws.String(bucketName),
		MaxKeys: aws.Int64(1000),
	}
	if len(prefix) != 0 {
		listObjectsInput.Prefix = aws.String(prefix)
	}
	for { // loop through all pages of the object list
		listObjectsOutput, err := s3Client.ListObjects(listObjectsInput)
		if err != nil {
			return objects, fmt.Errorf("Failed to list objects in bucket '%s': %w", bucketName, err)
		}
		// extract only the keys from each object
		for _, object := range listObjectsOutput.Contents {
			objects = append(objects, *object.Key)
		}
		// If no more pages, break out of the loop
		if !*listObjectsOutput.IsTruncated {
			break
		}
		listObjectsInput.Marker = listObjectsOutput.NextMarker
	}
	return objects, nil
}

func ObjectExists(config config.Config, key string) (bool, error) {
	// Get a list of objects that are prefixed by the target key
	objects, err := ListObjects(config, key)
	if err != nil {
		return false, err
	}
	// return true if we got a non-zero number of objects that match the prefix
	return len(objects) != 0, nil
}

func DeleteObject(config config.Config, key string) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return err
	}
	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}
	_, err = s3Client.DeleteObject(deleteObjectInput)
	if err != nil {
		return fmt.Errorf("Failed to delete object '%s' in bucket '%s': %w", key, bucketName, err)
	}
	return nil
}
