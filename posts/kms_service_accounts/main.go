package main

import (
	"context"
	"crypto"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"crypto/rand"

	"cloud.google.com/go/storage"
	salkmsauth "github.com/salrashid123/oauth2/google"
	salkmssign "github.com/salrashid123/signer/kms"
	"golang.org/x/oauth2"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	projectId           = "sd-test-246101"
	bucketName          = "sd-test-246101-bucket"
	kmsKeyRing          = "mycacerts"
	kmsKey              = "key1"
	kmsKeyVersion       = "1"
	kmsLocationId       = "us-central1"
	serviceAccountEmail = "kms-svc-account@sd-test-246101.iam.gserviceaccount.com"
	keyId               = "ce4ceffd5f9c8b399df9bf7b5c13327dab65f180"
	//keyId = "db8f0a5af9cf3bd211f4936ab7350788d4c774d8"
)

func main() {

	r, err := salkmssign.NewKMSCrypto(&salkmssign.KMS{
		ProjectId:  projectId,
		LocationId: kmsLocationId,
		KeyRing:    kmsKeyRing,
		Key:        kmsKey,
		KeyVersion: kmsKeyVersion,
	})

	if err != nil {
		log.Fatal(err)
	}

	bucket := bucketName
	object := "foo.txt"
	keyID := serviceAccountEmail
	expires := time.Now().Add(time.Minute * 10)

	s, err := storage.SignedURL(bucket, object, &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		GoogleAccessID: keyID,
		SignBytes: func(b []byte) ([]byte, error) {
			sum := sha256.Sum256(b)
			return r.Sign(rand.Reader, sum[:], crypto.SHA256)
		},
		Method:  "GET",
		Expires: expires,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println(s)

	resp, err := http.Get(s)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	log.Println("SignedURL Response :\n", string(body))
	if err != nil {
		log.Fatal(err)
	}

	// =============================================

	ts, err := salkmsauth.KmsTokenSource(
		&salkmsauth.KmsTokenConfig{
			Email:         serviceAccountEmail,
			ProjectId:     projectId,
			LocationId:    kmsLocationId,
			KeyRing:       kmsKeyRing,
			Key:           kmsKey,
			KeyVersion:    kmsKeyVersion,
			Audience:      "https://pubsub.googleapis.com/google.pubsub.v1.Publisher",
			KeyID:         keyId,
			UseOauthToken: true,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Transport: &oauth2.Transport{
			Source: ts,
		},
	}

	url := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics", projectId)
	resp, err = client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response: %v", resp.Status)

	ctx := context.Background()

	pubsubClient, err := pubsub.NewClient(ctx, projectId, option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("Could not create pubsub Client: %v", err)
	}

	it := pubsubClient.Topics(ctx)
	for {
		topic, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Unable to iterate topics %v", err)
		}
		log.Printf("Topic: %s", topic.ID())
	}

	// GCS does not support JWTAccessTokens, the following will only work if UseOauthToken is set to True
	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		log.Fatal(err)
	}
	sit := storageClient.Buckets(ctx, projectId)
	for {
		battrs, err := sit.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Printf(battrs.Name)
	}

}
