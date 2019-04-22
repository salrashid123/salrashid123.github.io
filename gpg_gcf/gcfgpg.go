package gcfgpg


import (

	"log"

	"os"
	"io"

	"context"
	"errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"cloud.google.com/go/logging"

	"cloud.google.com/go/storage"

	"google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	logName      = "go-functions"
	resourceType = "cloud_function"
)

var (
	password = []byte("helloworld")
	packetConfig = &packet.Config{
		DefaultCipher:  packet.CipherAES256,
	}
	
)

//Event represents Cloud Functions' incoming data struct
type Event struct {
	Bucket                  string `json:"bucket"`
	Name                    string `json:"name"`
	ContentType             string `json:"contentType"`
	Crc32c                  string `json:"crc32c"`
	Etag                    string `json:"etag"`
	Generation              string `json:"generation"`
	ID                      string `json:"id"`
	Kind                    string `json:"kind"`
	Md5Hash                 string `json:"md5Hash"`
	MediaLink               string `json:"mediaLink"`
	Metageneration          string `json:"metageneration"`
	SelfLink                string `json:"selfLink"`
	Size                    string `json:"size"`
	StorageClass            string `json:"storageClass"`
	TimeCreated             string `json:"timeCreated"`
	TimeStorageClassUpdated string `json:"timeStorageClassUpdated"`
	Updated                 string `json:"updated"`
}

type project struct {
	id       string
	function function
}
type function struct {
	name   string
	region string
	sink   string
}


func Encrypter(ctx context.Context, event Event) error {

    sink := os.Getenv("BUCKET_DST")
    if len(sink) == 0 {
        log.Fatal("Environment variable `BUCKET_DST` must be set and not empty.")
    }

    p := project{
        id: os.Getenv("GCLOUD_PROJECT"),
        function: function{
            name:   os.Getenv("FUNCTION_NAME"),
            region: os.Getenv("FUNCTION_REGION"),
            sink: sink,
        },
    }

    gcsClient, err := storage.NewClient(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer gcsClient.Close()

    logClient, err := logging.NewClient(ctx, p.id)
    if err != nil {
        log.Fatal(err)
    }
    defer logClient.Close()

	monitoredResource := monitoredres.MonitoredResource{
		Type: resourceType,
		Labels: map[string]string{
				"function_name": p.function.name,
				"region":        p.function.region,
		},
	}
	commonResource := logging.CommonResource(&monitoredResource)
	logger := logClient.Logger(logName, commonResource).StandardLogger(logging.Debug)

	logger.Printf("[Encrypter] Received: (%s) %s", event.Bucket, event.Name)

	srcBucket := gcsClient.Bucket(event.Bucket)

	dstBucket := gcsClient.Bucket(p.function.sink)

	gcsSrcObject := srcBucket.Object(event.Name)
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		logger.Fatal("[Encrypter] Error: (%s) ", err)
	}
	defer gcsSrcReader.Close()

	gcsDstObject := dstBucket.Object(event.Name + ".enc")
	gcsDstWriter := gcsDstObject.NewWriter(ctx)
	
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
	
		wm, err := armor.Encode(pw, "PGP MESSAGE", nil)
		if err != nil {
			logger.Fatal("[Encrypter] armor.Encode(pw,, nil): (%s) ", err)
		}
		pt, err := openpgp.SymmetricallyEncrypt(wm, password, nil, packetConfig)
		if err != nil {
			logger.Fatal("[Encrypter] openpgp.SymmetricallyEncrypt: (%s) ", err)
		}
		
		if _, err := io.Copy(pt, gcsSrcReader); err != nil {
			logger.Fatal("[Encrypter] Error io.Copy(pt, r.Body): (%s) ", err)
		}
		pt.Close()
		wm.Close()		
	}()

	n, err := io.Copy(gcsDstWriter, pr)
	if err != nil {
		logger.Fatal("[Encrypter] Error io.Copy(gcsDstWriter, encbuf): (%s) ", err)
	}

	err = gcsDstWriter.Close()
	if err != nil {
		logger.Fatal("[Encrypter] Error gcsDstWriter.Close: (%s) ", err)
	}
	logger.Printf("Encrypter: %d bytes are received.\n", n)
	return nil
}


func Decrypter(ctx context.Context, event Event) error {

    sink := os.Getenv("BUCKET_DST")
    if len(sink) == 0 {
        log.Fatal("Environment variable `BUCKET_DST` must be set and not empty.")
    }

    p := project{
        id: os.Getenv("GCLOUD_PROJECT"),
        function: function{
            name:   os.Getenv("FUNCTION_NAME"),
            region: os.Getenv("FUNCTION_REGION"),
            sink: sink,
        },
    }

    gcsClient, err := storage.NewClient(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer gcsClient.Close()

    logClient, err := logging.NewClient(ctx, p.id)
    if err != nil {
        log.Fatal(err)
    }
    defer logClient.Close()

	monitoredResource := monitoredres.MonitoredResource{
		Type: resourceType,
		Labels: map[string]string{
				"function_name": p.function.name,
				"region":        p.function.region,
		},
	}
	commonResource := logging.CommonResource(&monitoredResource)
	logger := logClient.Logger(logName, commonResource).StandardLogger(logging.Debug)

	logger.Printf("[Decrypter] Received: (%s) %s", event.Bucket, event.Name)

	srcBucket := gcsClient.Bucket(event.Bucket)

	dstBucket := gcsClient.Bucket(p.function.sink)

	gcsSrcObject := srcBucket.Object(event.Name)
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		logger.Fatal("[Decrypter] Error: (%s) ", err)
	}
	defer gcsSrcReader.Close()

	gcsDstObject := dstBucket.Object(event.Name + ".dec")
	gcsDstWriter := gcsDstObject.NewWriter(ctx)
	
	armorBlock, err := armor.Decode(gcsSrcReader)
	if err != nil {
		return nil
	}

	failed := false
	prompt := func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if failed {
			return nil, errors.New("decryption failed")
		}
		failed = true
		return password, nil
	}

	md, err := openpgp.ReadMessage(armorBlock.Body, nil, prompt, packetConfig)
	if err != nil {
		return  nil
	}
	
	n, err := io.Copy(gcsDstWriter,md.UnverifiedBody)
	if err != nil {
		log.Fatal(err)
	}
	err = gcsDstWriter.Close()
	if err != nil {
		log.Fatal(err)
	}
	
	logger.Printf("Decrypter: %d bytes are received.\n", n)
	return nil
}
