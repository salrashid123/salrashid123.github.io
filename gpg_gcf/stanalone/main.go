package main

import (
	"fmt"
	"log"
	"net/http"

	"io"
	"io/ioutil"
	"os"
	"time"

	//"context"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	//"cloud.google.com/go/storage"
)

var (
	gpgPassword     = []byte("helloworld")
	gpgPacketConfig = &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}
)

func encryptHandler(w http.ResponseWriter, r *http.Request) {

	gcsDstWriter, err := os.Create("./encrypted.txt")
	if err != nil {
		panic(err)
	}

	/*
		ctx := r.Context()
		gcsClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer gcsClient.Close()
		dstBucket := gcsClient.Bucket("YOURBUCKET")
		gcsDstObject := dstBucket.Object("encrypted.txt")
		gcsDstWriter := gcsDstObject.NewWriter(ctx)
	*/

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		wm, err := armor.Encode(pw, "PGP MESSAGE", nil)
		if err != nil {
			log.Fatal(err)
		}
		pt, err := openpgp.SymmetricallyEncrypt(wm, gpgPassword, nil, gpgPacketConfig)
		if err != nil {
			log.Fatal(err)
		}

		if _, err := io.Copy(pt, r.Body); err != nil {
			log.Fatal(err)
		}
		pt.Close()
		wm.Close()
	}()

	n, err := io.Copy(gcsDstWriter, pr)
	if err != nil {
		log.Fatal(err)
	}

	err = gcsDstWriter.Close()
	if err != nil {
		log.Fatal(err)
	}

	w.Write([]byte(fmt.Sprintf("%d bytes are received.\n", n)))
}

func decryptHandler(w http.ResponseWriter, r *http.Request) {

	gcsDstWriter, err := os.Create("./decrypted.txt")
	if err != nil {
		panic(err)
	}

	/*
		ctx := r.Context()
		gcsClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer gcsClient.Close()
		dstBucket := gcsClient.Bucket("YOURBUCKET")
		gcsDstObject := dstBucket.Object("result")
		gcsDstWriter := gcsDstObject.NewWriter(ctx)
	*/

	armorBlock, err := armor.Decode(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	failed := false
	prompt := func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if failed {
			log.Fatal(err)
		}
		failed = true
		return gpgPassword, nil
	}

	md, err := openpgp.ReadMessage(armorBlock.Body, nil, prompt, gpgPacketConfig)
	if err != nil {
		log.Fatal(err)
	}

	n, err := io.Copy(gcsDstWriter, md.UnverifiedBody)
	if err != nil {
		log.Fatal(err)
	}
	err = gcsDstWriter.Close()
	if err != nil {
		log.Fatal(err)
	}

	w.Write([]byte(fmt.Sprintf("%d bytes are received.\n", n)))
}

func upload(filename string, url string) {

	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	res, err := http.Post(url, "binary/octet-stream", file)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	message, _ := ioutil.ReadAll(res.Body)
	fmt.Printf(string(message))
}

func main() {

	http.HandleFunc("/encrypt", encryptHandler)
	http.HandleFunc("/decrypt", decryptHandler)

	go func() {
		time.Sleep(time.Second * 1)
		upload("./plain.txt", "http://127.0.0.1:8080/encrypt")
		//time.Sleep(time.Second * 10)
		//upload("./encrypted.txt", "http://127.0.0.1:8080/decrypt")
	}()

	log.Printf("Starting server")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
