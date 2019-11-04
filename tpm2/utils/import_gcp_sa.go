package main

/*
  Import a Google CLoud Service Account in a TPM.

  This utility is approximately the equivalent of the following (svc_account.p12 is a GCP service account):

	openssl pkcs12 -in svc_account.p12  -nocerts -nodes -passin pass:notasecret | openssl rsa -out private.pem
	openssl rsa -in privkey.pem -outform PEM -pubout -out public.pem
	tpm2_createprimary -C o -g sha256 -G rsa -c primary.ctx
	tpm2_import -C primary.ctx -G rsa -i private.pem -u key.pub -r key.prv
	tpm2_load -C primary.ctx -u key.pub -r key.prv -c key.ctx
	tpm2_evictcontrol -C o -c key.ctx 0x81010002


	go run main.go --serviceAccountFile svc_account.p12 --primaryFileName=primary.bin --keyFileName=key.bin --logtostderr=1 -v 10
*/

import (
	"flag"
	"fmt"
	"io/ioutil"

	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"

	"github.com/golang/glog"
	"github.com/google/go-tpm-tools/tpm2tools"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
	"golang.org/x/crypto/pkcs12"
)

const (
	defaultPassword = "notasecret"
	emptyPassword   = ""
)

var (
	handleNames = map[string][]tpm2.HandleType{
		"all":       []tpm2.HandleType{tpm2.HandleTypeLoadedSession, tpm2.HandleTypeSavedSession, tpm2.HandleTypeTransient},
		"loaded":    []tpm2.HandleType{tpm2.HandleTypeLoadedSession},
		"saved":     []tpm2.HandleType{tpm2.HandleTypeSavedSession},
		"transient": []tpm2.HandleType{tpm2.HandleTypeTransient},
	}

	defaultKeyParams = tpm2.Public{
		Type:    tpm2.AlgRSA,
		NameAlg: tpm2.AlgSHA256,
		Attributes: tpm2.FlagFixedTPM | tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin |
			tpm2.FlagUserWithAuth | tpm2.FlagRestricted | tpm2.FlagDecrypt,
		AuthPolicy: []byte{},
		RSAParameters: &tpm2.RSAParams{
			Symmetric: &tpm2.SymScheme{
				Alg:     tpm2.AlgAES,
				KeyBits: 128,
				Mode:    tpm2.AlgCFB,
			},
			KeyBits: 2048,
		},
	}

	tpmPath            = flag.String("tpm-path", "/dev/tpm0", "Path to the TPM device (character device or a Unix socket).")
	keyHandle          = flag.Int("handle", 0x81010002, "Handle value")
	serviceAccountFile = flag.String("serviceAccountFile", "", "ServiceAccount .p12 file")
	primaryFileName    = flag.String("primaryFileName", "", "Path to save the PrimaryContext.")
	keyFileName        = flag.String("keyFileName", "", "Path to save the KeyFile.")
)

func main() {

	flag.Parse()
	if *serviceAccountFile == "" {
		glog.Fatalf("serviceAccountFile pfx/p12 file must be specified")
	}

	glog.V(2).Infof("======= Init ========")

	glog.V(2).Infof("======= Converting ServiceAccount Key to PEM ========")
	data, err := ioutil.ReadFile(*serviceAccountFile)

	if err != nil {
		glog.Fatalf("     Unable to read serviceAccountFile %v", err)
	}

	privateKey, certificate, err := pkcs12.Decode(data, defaultPassword)
	if err != nil {
		glog.Fatalf("     Unable to Parse pkcs12 file %v", err)
	}

	pv := privateKey.(*rsa.PrivateKey)

	glog.V(8).Infof("     Certificate Subject %s", certificate.Subject)
	glog.V(8).Infof("     Public Key Size: %d", pv.Public().(*rsa.PublicKey).Size())

	rwc, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		glog.Fatalf("    can't open TPM %q: %v", tpmPath, err)
	}
	defer func() {
		if err := rwc.Close(); err != nil {
			glog.Fatalf("     %v\ncan't close TPM %q: %v", tpmPath, err)
		}
	}()

	glog.V(2).Infof("======= Flushing Transient Handles ========")
	totalHandles := 0
	for _, handleType := range handleNames["transient"] {
		handles, err := tpm2tools.Handles(rwc, handleType)
		if err != nil {
			glog.Fatalf("    getting handles: %v", err)
		}
		for _, handle := range handles {
			if err = tpm2.FlushContext(rwc, handle); err != nil {
				glog.Fatalf("    flushing handle 0x%x: %v", handle, err)
			}
			glog.V(2).Infof("    Handle 0x%x flushed\n", handle)
			totalHandles++
		}
	}

	glog.V(2).Infof("    %d handles flushed\n", totalHandles)

	glog.V(2).Infof("======= createPrimary ========")

	// // Test binding w/ PCR 23 auth
	// pcr := 23
	// pcrList := []int{23}
	// pcrval, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	// if err != nil {
	// 	glog.Fatalf("     Unable to  ReadPCR : %v", err)
	// }
	// glog.V(10).Infof("     PCR %v Value %v ", pcr, hex.EncodeToString(pcrval))

	// pcrSelection23 := tpm2.PCRSelection{Hash: tpm2.AlgSHA256, PCRs: pcrList}

	// but for now, just no selection
	pcrSelection23 := tpm2.PCRSelection{}

	kh, pub, err := tpm2.CreatePrimary(rwc, tpm2.HandleOwner, pcrSelection23, emptyPassword, emptyPassword, defaultKeyParams)
	if err != nil {
		glog.Fatalf("     CreatePrimary Failed: %v", err)
	}
	defer tpm2.FlushContext(rwc, kh)
	glog.V(10).Infof("    Primary KeySize %v", pub.(*rsa.PublicKey).Size())

	// reread the pub eventhough tpm2.CreatePrimary* gives pub
	tpmkPub, name, _, err := tpm2.ReadPublic(rwc, kh)
	if err != nil {
		glog.Fatalf("     ReadPublic failed: %s", err)
	}

	p, err := tpmkPub.Key()
	if err != nil {
		glog.Fatalf("      tpmPub.Key() failed: %s", err)
	}
	glog.V(10).Infof("     tpmPub Size(): %d", p.(*rsa.PublicKey).Size())

	b, err := x509.MarshalPKIXPublicKey(p)
	if err != nil {
		glog.Fatalf("     Unable to convert pub: %v", err)
	}

	kPubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: b,
		},
	)
	glog.V(10).Infof("     Pub Name: %v", hex.EncodeToString(name))
	glog.V(10).Infof("     PubPEM: \n%v", string(kPubPEM))

	if *primaryFileName != "" {

		glog.V(5).Infof("     ContextSave (%s) ========", *primaryFileName)
		ekhBytes, err := tpm2.ContextSave(rwc, kh)
		if err != nil {
			glog.Fatalf("     ContextSave failed for primaryFileName: %v", err)
		}
		err = ioutil.WriteFile("primary.bin", ekhBytes, 0644)
		if err != nil {
			glog.Fatalf("     ContextSave failed for primaryFileName: %v", err)
		}
		tpm2.FlushContext(rwc, kh)

		glog.V(5).Infof("     ContextLoad (%s) ========", *primaryFileName)
		khBytes, err := ioutil.ReadFile("primary.bin")
		if err != nil {
			glog.Fatalf("     ContextLoad failed for primaryFileName: %v", err)
		}
		kh, err = tpm2.ContextLoad(rwc, khBytes)
		if err != nil {
			glog.Fatalf("     ContextLoad failed for primaryFileName: %v", err)
		}
		defer tpm2.FlushContext(rwc, kh)
	}

	glog.V(2).Infof("======= Import ======= ")

	rp := tpm2.Public{
		Type:       tpm2.AlgRSA,
		NameAlg:    tpm2.AlgSHA256,
		Attributes: tpm2.FlagUserWithAuth | tpm2.FlagSign, // | tpm2.FlagDecrypt,
		RSAParameters: &tpm2.RSAParams{
			KeyBits:     2048,
			ExponentRaw: uint32(pv.PublicKey.E),
			ModulusRaw:  pv.PublicKey.N.Bytes(),
		},
	}
	rpriv := tpm2.Private{
		Type:      tpm2.AlgRSA,
		Sensitive: pv.Primes[0].Bytes(),
	}

	pubArea, err := rp.Encode()
	if err != nil {
		glog.Fatalf("     Public encoding error: %s", err)
	}

	decImported, err := tpm2.DecodePublic(pubArea)
	if err != nil {
		glog.Fatalf("     DecodePublic returned error: %v", err)
	}
	importedPubName, err := decImported.Name()
	glog.V(10).Infof("     Imported Public digestValue: %v", hex.EncodeToString(importedPubName.Digest.Value))

	privArea, err := rpriv.Encode()
	if err != nil {
		glog.Fatalf("     Private encoding error: %s", err)
	}

	duplicate, err := tpmutil.Pack(tpmutil.U16Bytes(privArea))
	if err != nil {
		glog.Fatalf("     Duplicate encoding error: %s", err)
	}

	emptyAuth := tpm2.AuthCommand{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession}

	iprivate, err := tpm2.Import(rwc, kh, emptyAuth, pubArea, duplicate, nil, nil, nil)
	if err != nil {
		glog.Fatalf("     Unable to Import Private: %v", err)
	}

	pH, name, err := tpm2.Load(rwc, kh, emptyPassword, pubArea, iprivate)
	if err != nil {
		glog.Fatalf("     Duplicate encoding error: %s", err)
	}
	defer tpm2.FlushContext(rwc, pH)
	glog.V(10).Infof("     Loaded Import Blob transient handle [0x%X], Name: %v", pH, hex.EncodeToString(name))

	if *keyFileName != "" {
		glog.V(5).Infof("     ContextSave (%s) ========", *keyFileName)
		pHBytes, err := tpm2.ContextSave(rwc, pH)
		if err != nil {
			glog.Fatalf("     ContextSave failed for key.bin: %v", err)
		}
		err = ioutil.WriteFile("key.bin", pHBytes, 0644)
		if err != nil {
			glog.Fatalf("     ContextSave failed for key.bin: %v", err)
		}
		tpm2.FlushContext(rwc, pH)

		glog.V(5).Infof("     ContextLoad (%s) ========", *keyFileName)
		pHBytes, err = ioutil.ReadFile("key.bin")
		if err != nil {
			glog.Fatalf("     ContextLoad failed for importedKey: %v", err)
		}
		pH, err = tpm2.ContextLoad(rwc, pHBytes)
		if err != nil {
			glog.Fatalf("     ContextLoad failed for importedKey: %v", err)
		}
		defer tpm2.FlushContext(rwc, pH)
	}

	glog.V(2).Infof("======= EvictControl ======== ")

	glog.V(5).Infof("     Evicting Persistent Handle at %v ", fmt.Sprintf("%x", *keyHandle))
	pHandle := tpmutil.Handle(*keyHandle)
	err = tpm2.EvictControl(rwc, emptyPassword, tpm2.HandleOwner, pHandle, pHandle)
	if err != nil {
		glog.Fatalf("     Unable evict persistentHandle: %v", err)
	}

	err = tpm2.EvictControl(rwc, emptyPassword, tpm2.HandleOwner, pH, pHandle)
	if err != nil {
		glog.Fatalf("     Unable to set persistentHandle: %v", err)
	}
	defer tpm2.FlushContext(rwc, pHandle)
	glog.V(2).Infof("     ServiceAccount key persisted")

	// dataToSign := []byte("secret")
	// digest := sha256.Sum256(dataToSign)
	// sig, err := tpm2.Sign(rwc, pHandle, emptyPassword, digest[:], &tpm2.SigScheme{
	// 	Alg:  tpm2.AlgRSASSA,
	// 	Hash: tpm2.AlgSHA256,
	// })
	// if err != nil {
	// 	glog.Fatalf("Error Signing: %v", err)
	// }

	// glog.V(2).Infof("Signature data:  %s", base64.RawStdEncoding.EncodeToString([]byte(sig.RSA.Signature)))

	// if err := rsa.VerifyPKCS1v15(pv.Public().(*rsa.PublicKey), crypto.SHA256, digest[:], []byte(sig.RSA.Signature)); err != nil {
	// 	glog.Fatalf("Signature verification failed: %v", err)
	// }

}
