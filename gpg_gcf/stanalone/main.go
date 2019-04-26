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
	bucketName = "mineral-minutia-820-mlengine"
)

func encryptHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsClient.Close()

	srcBucket := gcsClient.Bucket(bucketName)
	gcsSrcObject := srcBucket.Object("plain.txt")
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsSrcReader.Close()

	dstBucket := gcsClient.Bucket(bucketName)
	gcsDstObject := dstBucket.Object("encrypted.txt")
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


	ctx := r.Context()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer gcsClient.Close()

	srcBucket := gcsClient.Bucket(bucketName)
	gcsSrcObject := srcBucket.Object("encrypted.txt")
	gcsSrcReader, err := gcsSrcObject.NewReader(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer gcsSrcReader.Close()

	dstBucket := gcsClient.Bucket(bucketName)
	gcsDstObject := dstBucket.Object("decrypted_plain.txt")
	gcsDstWriter := gcsDstObject.NewWriter(ctx)
	

	armorBlock, err := armor.Decode(gcsSrcReader)
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
