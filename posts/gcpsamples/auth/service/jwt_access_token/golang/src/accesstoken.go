// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// This sample demonstrates AccessTokenCredentials:
// https://godoc.org/golang.org/x/oauth2/google#JWTAccessTokenSourceFromJSON

// To use, create a service accountJSON file and allow it atleast Pub/Sub Viewer IAM
// permissions to list Topics on a project.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	projectID = flag.String("project", "", "Project ID")
	keyfile   = flag.String("keyfile", "", "Service Account JSON keyfile")
)

func main() {
	flag.Parse()

	if *projectID == "" {
		fmt.Fprintln(os.Stderr, "missing -project flag")
		flag.Usage()
		os.Exit(2)
	}
	if *keyfile == "" {
		fmt.Fprintln(os.Stderr, "missing -keyfile flag")
		flag.Usage()
		os.Exit(2)
	}

	// audience values for other services can be found in the repo here similar to
	// PubSub
	// https://github.com/googleapis/googleapis/blob/master/google/pubsub/pubsub.yaml
	var aud string = "https://pubsub.googleapis.com/google.pubsub.v1.Publisher"

	ctx := context.Background()
	keyBytes, err := ioutil.ReadFile(*keyfile)
	if err != nil {
		glog.Error("Unable to read service account key file  %v", err)
	}

	start := time.Now()
	tokenSource, err := google.JWTAccessTokenSourceFromJSON(keyBytes, aud)
	if err != nil {
		glog.Error("Error building JWT access token source: %v", err)
	}

	/*
		jwt, err := tokenSource.Token()
		if err != nil {
			glog.Error("Unable to generate JWT token: %v", err)
		}
		glog.V(3).Infoln(jwt.AccessToken)
	*/

	client, err := pubsub.NewClient(ctx, *projectID, option.WithTokenSource(tokenSource))
	if err != nil {
		glog.Error("Could not create pubsub Client: %v", err)
	}
	topics := client.Topics(ctx)
	for {
		topic, err := topics.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			glog.Error("Error listing topics %v", err)
		}
		glog.V(3).Infoln(topic)
	}
	elapsed := time.Since(start)
	fmt.Println(math.Round(elapsed.Seconds() * 1000))
}
