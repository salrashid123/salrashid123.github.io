package gcffan

import (
	"log"

	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/logging"

	"github.com/google/uuid"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"

	"google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	logName      = "go-functions"
	resourceType = "cloud_function"
	locationID = "us-central1"
	queueID    = "q1"
)

var ()

type Event struct {
	Kind                    string    `json:"kind"`
	ID                      string    `json:"id"`
	SelfLink                string    `json:"selfLink"`
	Name                    string    `json:"name"`
	Bucket                  string    `json:"bucket"`
	Generation              string    `json:"generation"`
	Metageneration          string    `json:"metageneration"`
	ContentType             string    `json:"contentType"`
	TimeCreated             time.Time `json:"timeCreated"`
	Updated                 time.Time `json:"updated"`
	TemporaryHold           bool      `json:"temporaryHold"`
	EventBasedHold          bool      `json:"eventBasedHold"`
	RetentionExpirationTime time.Time `json:"retentionExpirationTime"`
	StorageClass            string    `json:"storageClass"`
	TimeStorageClassUpdated time.Time `json:"timeStorageClassUpdated"`
	Size                    string    `json:"size"`
	Md5Hash                 string    `json:"md5Hash"`
	MediaLink               string    `json:"mediaLink"`
	ContentEncoding         string    `json:"contentEncoding"`
	ContentDisposition      string    `json:"contentDisposition"`
	CacheControl            string    `json:"cacheControl"`

	Metadata map[string]interface{} `json:"metadata"`
	Acl      []struct {
		Kind       string `json:"kind"`
		ID         string `json:"id"`
		SelfLink   string `json:"selfLink"`
		Bucket     string `json:"bucket"`
		Object     string `json:"object"`
		Generation string `json:"generation"`
		Entity     string `json:"entity"`
		Role       string `json:"role"`
		Email      string `json:"email"`
		EntityId   string `json:"entityId"`
		Domain     string `json:"domain"`

		ProjectTeam struct {
			ProjectNumber string `json:"projectNumber"`
			Team          string `json:"team"`
		} `json:"projectTeam"`
		Etag string `json:"etag"`
	} `json:"acl"`

	Owner struct {
		Entity   string `json:"entity"`
		EntityId string `json:"entityId"`
	}
	Crc32c             string `json:"crc32c"`
	ComponentCount     int    `json:"componentCount"`
	Etag               string `json:"etag"`
	CustomerEncryption struct {
		EncryptionAlgorithm string `json:"encryptionAlgorithm"`
		KeySha256           string `json:"keySha256"`
	}
	KmsKeyName string `json:"kmsKeyName"`
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

func Fan(ctx context.Context, event Event) error {

	p := project{
		id: os.Getenv("GCLOUD_PROJECT"),
		function: function{
			name:   os.Getenv("FUNCTION_NAME"),
			region: os.Getenv("FUNCTION_REGION"),
		},
	}
	logClient, err := logging.NewClient(ctx, p.id)
	if err != nil {
		log.Fatal(err)
	}
	defer logClient.Close()

	cloudRunURL := os.Getenv("RUN_URL")
	serviceAccount := os.Getenv("SERVICE_ACCOUNT")

	monitoredResource := monitoredres.MonitoredResource{
		Type: resourceType,
		Labels: map[string]string{
			"function_name": p.function.name,
			"region":        p.function.region,
		},
	}
	commonResource := logging.CommonResource(&monitoredResource)
	logger := logClient.Logger(logName, commonResource).StandardLogger(logging.Debug)

	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}

	destName := fmt.Sprintf("%s", event.Metadata["name"])
	destFormats := event.Metadata["formats"]
	s := strings.Split(fmt.Sprintf("%v", destFormats), ",")

	for _, destFmt := range s {
		data := url.Values{}
		data.Set("format", destFmt)
		data.Set("source", event.Name)
		data.Set("name", destName+"."+destFmt)

		logger.Printf("Converting source [%s], to [%s] as format [%s]", event.Name, destName, destFmt)


		taskID := uuid.New().String()

		parentName := fmt.Sprintf("projects/%s/locations/%s/queues/%s", os.Getenv("GCLOUD_PROJECT"), locationID, queueID)
		queuePath := fmt.Sprintf("%s/tasks/%s", parentName, taskID)
	

		url := cloudRunURL + "/convert"
		req := &taskspb.CreateTaskRequest{
			Parent: parentName,
			Task: &taskspb.Task{
				Name: queuePath,
				MessageType: &taskspb.Task_HttpRequest{
					HttpRequest: &taskspb.HttpRequest{
						HttpMethod: taskspb.HttpMethod_POST,
						Url:        url,
						Headers:    map[string]string{"Content-type": "application/x-www-form-urlencoded"},
						Body:       []byte(data.Encode()),
						AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
							OidcToken: &taskspb.OidcToken{
								ServiceAccountEmail: serviceAccount,
								Audience:            cloudRunURL,
							},
						},
					},
				},
			},
		}
		resp, err := client.CreateTask(ctx, req)
		if err != nil {
			logger.Fatalf("ERROR: %v", err.Error())
		}
		logger.Printf("Enqueued Task %s", resp.GetName())
	}

	return nil
}
