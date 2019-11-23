# Yet another image file converter on GCP

Two workflows to convert an images to different formats on GCP and optionally save on [Cloud CDN](https://cloud.google.com/cdn/)

1. Inline

- Inline:  upload directly, convert file; either return converted file in respose or save to GCS and provide CDN URL 

`client -> [upload image in POST, specify conversion specs as query-param] --> Cloud Run converts image --> Saves converted file to GCS`


2. Batch

- Upload images files to source GCS Bucket.  Specify file formats to convert to as metadata.  Converted files saved to GCS

`client --> GCS --> [Object Change Notification] --> GCF --> [for each file format, create Cloud Task] --> Cloud Task --> Cloud Run converts image --> Saves converted file to GCS.`


both mode uses - [https://github.com/gographics/imagick](https://github.com/gographics/imagick)


### Setup Env Variables

```
export PROJECT_ID=`gcloud config get-value core/project`
export PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format='get(projectNumber)')"
export DEST_BUCKET_NAME=$PROJECT_ID-cdn
export CDN_URL=https://images.esodemoapp2.com
export DEST_BUCKET_NAME=$PROJECT_ID-cdn
export SRC_BUCKET_NAME=$PROJECT_ID-img-src

gcloud services enable compute.googleapis.com run.googleapis.com \
   cloudfunctions.googleapis.com \
   cloudtrace.googleapis.com logging.googleapis.com \
   monitoring.googleapis.com  cloudtasks.googleapis.com
```

### CDN

Create GCS bucket that is enabled for Cloud CDN.

ref [Using a Cloud Storage bucket as a load balancer backend](https://cloud.google.com/load-balancing/docs/backend-bucket)


You can use the canned ssl certificates in this repo.

```bash
gsutil mb gs://$DEST_BUCKET_NAME

gcloud compute addresses create image-cdn --global

gcloud compute ssl-certificates create cdn-cert --certificate=images.crt --private-key=images.key

gcloud compute backend-buckets create static-bucket --gcs-bucket-name $DEST_BUCKET_NAME --enable-cdn

gcloud compute url-maps create cdn-map  --default-backend-bucket=static-bucket


gcloud compute target-https-proxies create cdn-lb-proxy --url-map=cdn-map --ssl-certificates=cdn-cert --global
gcloud compute forwarding-rules create cdn-content-rule --address  35.244.244.107 --global --target-https-proxy cdn-lb-proxy --ports 443
```


List the IP address (you'll use is if you want to serve the images from CDN)

```bash
$  gcloud compute addresses list
NAME                              ADDRESS/RANGE    TYPE      PURPOSE       NETWORK  REGION       SUBNET   STATUS
image-cdn                         35.244.244.107   EXTERNAL                                               RESERVED
```


## Inline

Configure Inline processing


Note the
* Cloud Run Service Account Format: `$PROJECT_NUMBER-compute@developer.gserviceaccount.com`

Allow that service account to write Metrics, Logs and Trace
```
gcloud projects add-iam-policy-binding $PROJECT_ID  \
   --member=serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com \
   --role=roles/logging.logWriter

gcloud projects add-iam-policy-binding $PROJECT_ID  \
   --member=serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com \
   --role=roles/monitoring.metricWriter

gcloud projects add-iam-policy-binding $PROJECT_ID  \
   --member=serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com \
   --role=roles/cloudtrace.agent
```

Grant the Service account Cloud Run uses permissions to write to the destination bucket

```
gsutil iam ch serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com:objectCreator,objectViewer,objectAdmin gs://$DEST_BUCKET_NAME
```


Build the Docker image
```
docker build -t gcr.io/$PROJECT_ID/imagetx .
```

If you want to run the image locally, create a service account and grant the same IAM permissions as given to cloud Run (save the json file as `svc_account.json`)

```
docker run -v /path/to/service_account_file/:/certs \
  -e PROJECT_ID=$PROJECT_ID -e CDN_URL=$CDN_URL \
  -e DEST_BUCKET_NAME=$DEST_BUCKET_NAME \
  -e GOOGLE_APPLICATION_CREDENTIALS=/certs/svc_account.json \
  -p 8080:8080 gcr.io/$PROJECT_ID/imagetx
```

Upload the file to gcr:

```
gcloud auth configure-docker
docker push gcr.io/$PROJECT_ID/imagetx
```

Deploy toCloud Run
```
gcloud config set run/platform managed
gcloud config set run/region us-central1

gcloud beta run deploy imagex --image=gcr.io/$PROJECT_ID/imagetx \
  --set-env-vars=PROJECT_ID=$PROJECT_ID,DEST_BUCKET_NAME=$DEST_BUCKET_NAME,CDN_URL=$CDN_URL \
  --allow-unauthenticated
```

Note the Cloud RUN URL


Use any image file locally and POST to cloud run as form encoded:

In the following case `logo.png` will get converted to jpg and saved as `logo2.jpg`

```
 curl -s -X POST -F 'name=logo2' -F 'format=jpg' -F 'image=@logo.png'  https://imagex-avceceujcq-uc.a.run.app/convert
```

The default application under `convert_stream/src/main.go` simply returns a URL for the CDN.  If you actually want the image returned inline, do not specify the `CDN_URL` environment variable during deployment.  The default applciation does not mark the uploaded image as public though i have code there commentd out that does that.


## BATCH


`GCS --> GCF --> Tasks --> Run --> GCS`


Note the Service Account formats used here:

* Cloud Run SA: `$PROJECT_NUMBER-compute@developer.gserviceaccount.com`
* Cloud Functions SA: `$PROJECT_ID@appspot.gserviceaccount.com`
* Tasks SA: `service-$PROJECT_NUMBER@gcp-sa-cloudtasks.iam.gserviceaccount.com`


Create the _Source_ GCS Bucket.  This bucket will hold the images you upload that you intend to convert.

```
gsutil mb gs://$SRC_BUCKET_NAME
```

Build and Deploy Cloud Run

```
cd convert_batch/

docker build -t gcr.io/$PROJECT_ID/imagebatch .
docker push gcr.io/$PROJECT_ID/imagebatch


gcloud beta run deploy imagebatch --image=gcr.io/$PROJECT_ID/imagebatch --no-allow-unauthenticated \
  --set-env-vars=PROJECT_ID=$PROJECT_ID,SRC_BUCKET_NAME=$SRC_BUCKET_NAME,DEST_BUCKET_NAME=$DEST_BUCKET_NAME,CDN_URL=$CDN_URL
```

Note down the Cloud RUN URL and export it (eg)
```
export RUN_URL=https://imagebatch-avceceujcq-uc.a.run.app
```

Allow Run to read Source GCS Bucket
```
  gsutil iam ch serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com:objectViewer gs://$SRC_BUCKET_NAME
```

Allow Run to write to DEST GCS Bucket
```
  gsutil iam ch serviceAccount:$PROJECT_NUMBER-compute@developer.gserviceaccount.com:objectCreator gs://$DEST_BUCKET_NAME
```

Deploy TaskQueue:
```
gcloud app deploy  queue.yaml

   (select us-central1)
```




Allow GCF to enqueue Task
```
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member serviceAccount:$PROJECT_ID@appspot.gserviceaccount.com \
  --role roles/cloudtasks.enqueuer
```

Allow Tasks use OIDC TOken AS GCF

```
gcloud iam service-accounts  add-iam-policy-binding \
   $PROJECT_ID@appspot.gserviceaccount.com \
   --member=serviceAccount:service-$PROJECT_NUMBER@gcp-sa-cloudtasks.iam.gserviceaccount.com  --role=roles/cloudtasks.serviceAgent
```


Deploy GCF function invoked by GCS

```
gcloud  functions deploy Fan --entry-point=Fan --runtime go111 \
  --trigger-resource=${SRC_BUCKET_NAME} --set-env-vars=RUN_URL=${RUN_URL},SERVICE_ACCOUNT=$PROJECT_ID@appspot.gserviceaccount.com \
  --trigger-event=google.storage.object.finalize --project=${PROJECT_ID}
```


- Allow GCF OIDC Token to invoke Run

  gcloud run services add-iam-policy-binding imagebatch \
    --member="serviceAccount:$PROJECT_ID@appspot.gserviceaccount.com" \
    --role="roles/run.invoker"


Upload file to convert to the Destination BUcket

```
 gsutil -h "x-goog-meta-name:1" -h "x-goog-meta-formats:svg,jpg" cp 1.png gs://$SRC_BUCKET_NAME/
```

What you should see in the source bucket is the list uploaded file

![images/src_bucket.png](images/src_bucket.png)

For each file format specified (`svg`,`png`) a new converted file will appear

![images/dest_bucket.png](images/dest_bucket.png)



### Monitoring Metrics

Cloud Run was configured to emit custom metrics and traces.   The specific metric that is captures is the size of the image file that was processed and the file type.  You can see the metric in Stackdriver as  

```json
    {
      "name": "projects/img-test-259714/metricDescriptors/custom.googleapis.com/opencensus/mBytes/bytes",
      "labels": [
        {
          "key": "filetype"
        }
      ],
      "metricKind": "CUMULATIVE",
      "valueType": "DISTRIBUTION",
      "unit": "By",
      "description": "The number of bytes processed",
      "displayName": "OpenCensus/mBytes/bytes",
      "type": "custom.googleapis.com/opencensus/mBytes/bytes",
      "monitoredResourceTypes": [
        "aws_ec2_instance",
        "cloud_composer_environment",
        "cloud_composer_workflow",
        "dataflow_job",
        "gce_instance",
        "generic_node",
        "generic_task",
        "gke_container",
        "global",
        "k8s_cluster",
        "k8s_container",
        "k8s_node",
        "k8s_pod",
        "knative_revision"
      ]
    },
```

and in stackdriver console as heatmap

![images/std_files.png](images/std_files.png)


### Tracing

Cloud Run is also enabled for cloud Trace measurements.  You can find those traces on the GCP console and shows the request spans you define _and_ the api calls to GCP

![images/trace.png](images/trace.png)


## Conclusion

i don't have any, its just a sample


#### TODO:

* make dockerfile smaller...

# REFERENCES

just some references

- [Google Cloud Storage “exploder” #2](https://medium.com/google-cloud/google-cloud-storage-exploder-2-9870d41fcee3)
- [GPG stream encryption and decryption on Google Cloud Functions and Cloud Run](https://medium.com/google-cloud/gpg-stream-encryption-and-decryption-on-google-cloud-functions-and-cloud-run-8cb732404134)
- [Anti Virus file scanning on Google Cloud Storage using ClamAV](https://medium.com/google-cloud/tutorial-on-how-to-use-clamav-to-scan-files-uploaded-to-google-cloud-storage-gcs-b37777360210)
- [https://github.com/GoogleCloudPlatform/golang-samples/blob/master/functions/imagemagick/imagemagick.go](https://github.com/GoogleCloudPlatform/golang-samples/blob/master/functions/imagemagick/imagemagick.go)
- [https://medium.com/@didil/image-resizing-with-go-and-cloud-functions-792a47e6473d](https://medium.com/@didil/image-resizing-with-go-and-cloud-functions-792a47e6473d)
- [https://medium.com/learning-the-go-programming-language/streaming-io-in-go-d93507931185](https://medium.com/learning-the-go-programming-language/streaming-io-in-go-d93507931185)
