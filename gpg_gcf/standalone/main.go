package main

import (
	"fmt"
	"log"
	"net/http"

	"io"

	//"context"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"cloud.google.com/go/storage"
)

var (
	gpgPassword     = []byte("helloworld")
	gpgPacketConfig = &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}
	bucketName = "YOURBUCKETNAME"
)

func encryptHandler(w http.ResponseWriter, r *http.Request) {

	fileName := r.URL.Query().Get("file")
	if fileName != "" {
		fileName = "plain.txt"
	}

	ctx := r.Context()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsClient.Close()

	// read source file from GCS
	srcBucket := gcsClient.Bucket(bucketName)
	gcsSrcObject := srcBucket.Object(fileName)
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsSrcReader.Close()

	dstBucket := gcsClient.Bucket(bucketName)
	gcsDstObject := dstBucket.Object(fileName + ".enc")
	gcsDstWriter := gcsDstObject.NewWriter(ctx)
	
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		wm, err := armor.Encode(pw, "PGP MESSAGE", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		pt, err := openpgp.SymmetricallyEncrypt(wm, gpgPassword, nil, gpgPacketConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if _, err := io.Copy(pt, gcsSrcReader); err != nil {
		// or read the request body to decrypt
		//if _, err := io.Copy(pt, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		pt.Close()
		wm.Close()
	}()

	n, err := io.Copy(gcsDstWriter, pr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = gcsDstWriter.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Write([]byte(fmt.Sprintf("%d bytes are received.\n", n)))
}

func decryptHandler(w http.ResponseWriter, r *http.Request) {

	fileName := r.URL.Query().Get("file")
	if fileName != "" {
		fileName = "plain.txt.enc"
	}

	ctx := r.Context()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer gcsClient.Close()

	// read source file from GCS
	srcBucket := gcsClient.Bucket(bucketName)
	gcsSrcObject := srcBucket.Object(fileName)
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsSrcReader.Close()

	dstBucket := gcsClient.Bucket(bucketName)
	gcsDstObject := dstBucket.Object(fileName + ".dec")
	gcsDstWriter := gcsDstObject.NewWriter(ctx)
	

	armorBlock, err := armor.Decode(gcsSrcReader)

	// or read the request body to decrypt
	//armorBlock, err := armor.Decode(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	failed := false
	prompt := func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if failed {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		failed = true
		return gpgPassword, nil
	}

	md, err := openpgp.ReadMessage(armorBlock.Body, nil, prompt, gpgPacketConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	n, err := io.Copy(gcsDstWriter, md.UnverifiedBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = gcsDstWriter.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Write([]byte(fmt.Sprintf("%d bytes are received.\n", n)))
}


func main() {

	http.HandleFunc("/encrypt", encryptHandler)
	http.HandleFunc("/decrypt", decryptHandler)

	log.Printf("Starting server")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
