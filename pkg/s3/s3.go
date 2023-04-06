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
	"github.com/trailofbits/cloudexec/pkg/config"
)

/*
 * the bucket hub, everything related to s3-style buckets of files
 * exports the following functions:
 * - ListBuckets(config config.Config) ([]string, error)
 * - CreateBucket(config config.Config, bucket string) error
 * - PutObject(config config.Config, bucket string, key string, value []byte) error
 * - GetObject(config config.Config, bucket string, key string) ([]byte, error)
 * - ListObjects(config config.Config, bucket string, prefix string) ([]string, error)
 * - DeleteObject(config config.Config, bucket string, key string) error
 */

var s3Client *s3.S3

// Note: not safe to use concurrently from multiple goroutines (yet)
func initializeS3Client(config config.Config, init bool) (*s3.S3, error) {
	// Immediately return our cached client if available
	if s3Client != nil {
		return s3Client, nil
	}

	// Unpack required config values
	spacesAccessKey := config.DigitalOcean.SpacesAccessKey
	spacesSecretKey := config.DigitalOcean.SpacesSecretKey
	endpointRegion := config.DigitalOcean.SpacesRegion

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
		Endpoint:         aws.String(fmt.Sprintf("https://%s.digitaloceanspaces.com", endpointRegion)),
		Region:           aws.String(spacesRegion),
		S3ForcePathStyle: aws.Bool(false),
	}

	// Create a new session and S3 client
	newSession, err := session.NewSession(spacesConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create S3 client: %w", err)
	}

	// Cache client for subsequent usage iff not the client used for initialization
	if !init {
		s3Client = s3.New(newSession)
	}

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

func GetOrCreateBucket(config config.Config, username string) error {
	// TODO: sanitize username & centralize bucket name creation
	bucket := fmt.Sprintf("cloudexec-%s-trailofbits", username)

	listBucketsOutput, err := ListBuckets(config)
	if err != nil {
		return fmt.Errorf("Failed to list buckets: %w", err)
	}

	// Check if the desired Space already exists
	for _, thisBucket := range listBucketsOutput {
		if thisBucket == bucket {
			// create a non-init client
			s3Client, err := initializeS3Client(config, false)
			if err != nil {
				return err
			}

			// ensure versioning is enabled on the bucket
			_, err = s3Client.PutBucketVersioning(&s3.PutBucketVersioningInput{
				Bucket: aws.String(bucket),
				VersioningConfiguration: &s3.VersioningConfiguration{
					Status: aws.String("Enabled"),
				},
			})
			if err != nil {
				return fmt.Errorf("Failed to enable versioning on bucket '%s': %w", bucket, err)
			}
			return nil
		}
	}

	// create an initialization client
	s3Client, err := initializeS3Client(config, true)
	if err != nil {
		return err
	}

	// craft bucket creation request
	createBucketInput := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	// execution bucket creation
	_, err = s3Client.CreateBucket(createBucketInput)
	if err != nil {
		return fmt.Errorf("Failed to create bucket '%s': %w", bucket, err)
	}

	// wait for the bucket to be available
	err = s3Client.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("Failed to wait for bucket '%s': %w", bucket, err)
	}

	// enable versioning on the bucket
	_, err = s3Client.PutBucketVersioning(&s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &s3.VersioningConfiguration{
			Status: aws.String("Enabled"),
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to enable versioning on bucket '%s': %w", bucket, err)
	}

	fmt.Printf("Created bucket '%s'...\n", bucket)
	return nil
}

func PutObject(config config.Config, bucket string, key string, value []byte) error {
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return err
	}

	// If zero-length value is given, create a directory instead of a file
	if len(value) == 0 {
		// Create the state directory
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String("state/"),
		})
		if err != nil {
			return fmt.Errorf("Failed to create %s directory in bucket %s: %w", key, bucket, err)
		}
		return nil
	}

	// hash the input to ensure the integrity of file
	// Does the s3 sdk do this for us automatically?
	md5Hash := md5.Sum(value)
	md5HashBase64 := base64.StdEncoding.EncodeToString(md5Hash[:])

	// Upload the file
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(bytes.NewReader(value)),
		ACL:         aws.String("private"),
		ContentType: aws.String(http.DetectContentType(value)),
		ContentMD5:  aws.String(md5HashBase64),
	})
	if err != nil {
		return fmt.Errorf("Failed to upload file %s to bucket %s: %w", key, bucket, err)
	}
	// fmt.Printf("Successfully uploaded %s to %s\n", key, bucket)

	return nil
}

func GetObject(config config.Config, bucket string, key string) ([]byte, error) {
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return []byte{}, err
	}

	const maxRetries = 3
	for i := 1; i <= maxRetries; i++ {
		resp, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
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

func DeleteObject(config config.Config, bucket string, key string) error {
	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return err
	}

	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err = s3Client.DeleteObject(deleteObjectInput)
	if err != nil {
		return fmt.Errorf("Failed to delete object '%s' in bucket '%s': %w", key, bucket, err)
	}

	return nil
}

func ListObjects(config config.Config, bucket string, prefix string) ([]string, error) {
	var objects []string

	// create a client
	s3Client, err := initializeS3Client(config, false)
	if err != nil {
		return objects, err
	}

	var listObjectsInput *s3.ListObjectsInput
	if len(prefix) == 0 {
		// List all objects in the bucket
		listObjectsInput = &s3.ListObjectsInput{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int64(1000),
		}
	} else {
		// List all objects with keys that begin with the given prefix
		listObjectsInput = &s3.ListObjectsInput{
			Bucket:  aws.String(bucket),
			Prefix:  aws.String(prefix),
			MaxKeys: aws.Int64(1000),
		}
	}

	listObjectsOutput, err := s3Client.ListObjects(listObjectsInput)
	if err != nil {
		return objects, fmt.Errorf("Failed to list objects in bucket '%s': %w", bucket, err)
	}

	// extract just the keys from each object
	for _, object := range listObjectsOutput.Contents {
		objects = append(objects, *object.Key)
	}

	return objects, nil
}
