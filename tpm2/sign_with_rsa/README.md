
# TPM Sign with RSA

```
tpm2_createprimary -C e -c primary.ctx
tpm2_create -G rsa -u key.pub -r key.priv -C primary.ctx

tpm2_load -C primary.ctx -u key.pub -r key.priv -c key.ctx

echo "my message" > message.dat
tpm2_sign -c key.ctx -g sha256 -o sig.rssa message.dat
tpm2_verifysignature -c key.ctx -g sha256 -s sig.rssa -m message.dat
```


---

```
// # go run main.go  --logtostderr=1 -v 5
// I1028 23:28:50.890079    9664 main.go:34] ======= Init  ========
// I1028 23:28:50.905537    9664 main.go:61] 0 handles flushed
// I1028 23:28:50.911022    9664 main.go:77] Signature data:  yt5cQcZt
```