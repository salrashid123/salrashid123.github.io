# Trusted Platform Module (TPM) recipes with tpm2_tools and go-tpm

Just some sample common flows for use with TPM modules and libraries.

The primary focus is how use `tpm2_tools` to perform common tasks that i've come across.

Also shown equivalent use of `go-tpm` library set.


- [tpm2-tools](https://github.com/tpm2-software/tpm2-tools)
- [go-tpm](https://github.com/google/go-tpm)
- [go-tpm-tools](https://github.com/google/go-tpm-tools)
- [go-attestation](https://github.com/google/go-attestation)


## Usecases:

- `encrypt_with_tpm_rsa`: Encrypt with RSA Key generated on TPM (`tpm2_create`, `tpm2_rsaencrypt, tpm2_decrypt`)

- `tpm_quote_verify`: Generate TPM Quote blob with PCR23 value (`tpm2_createak`, `tpm2_quote`, `tpm2_checkquote`)

- `seal_to_tpm`: Seal arbitrary data directly to TPM; use PCR Policy (`tpm2_create`, `tpm2_load`, `tpm2_seal`, `tpm2_unseal`)

- `sign_with_ak`: Sign with Attestation Key (`tpm2_createak`, `tpm2_hash`, `tpm2_sign`, `tpm2_verifysignature`)

- `sign_wth_rsa`: Generate RSA key with TPM  and sign (`tpm2_create`,`tpm2_load`, `tpm2_sign`, `tpm2_verifysignature`)

- `tpm_import_external_rsa`: Import external RSA key to TPM; decrypt data with TPM (`tpm2_import, tpm2_load, tpm2_rsadecrypt`)

- `tpm_make_activate`: Attestation Protocol using Make-Activate credentials (`tpm2_makecredential`, `tpm2_activatecredential`)

- `tpm2_duplicate`: Use (`tpm2_import`, `tpm2_duplicate`) encrypt and transfer a key from one TPM to another.

- `ek_import_blob`: Transfer secret from one TPM to another using ekPub only. Example only covers `go-tpm` based transfer (TODO: figure out the `tpm2_tools` flow).

- `utils`:  Utility functions
    - Import Google Cloud Service account .p12 file into a TPM as a persistent handle.
    - Convert PEM formatted key to TPM2-tools format.

### Usage

Excercising any of the scenarios above requires access to a TPM(!).  You can use `vTPM` included with a Google Cloud [Shielded VM](https://cloud.google.com/shielded-vm/) surfaced at `/dev/tpm0` on the VM

#### Create test VM with TPM

```
 gcloud  compute  instances create shielded-1 --zone=us-central1-a --machine-type=n1-standard-1 --no-service-account --no-scopes --image=ubuntu-1804-bionic-v20191002 --image-project=gce-uefi-images --no-shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring
```

> note, you may need to update `--image=`

#### Installing tpm2_tools, golang

- Install golang

- Install `tpm2_tools`:

  - [tpm2-tss/INSTALL](https://github.com/tpm2-software/tpm2-tss/blob/master/INSTALL.md)
  - [tpm2-tools/INSTALL](https://github.com/tpm2-software/tpm2-tools/blob/master/INSTALL.md)

```bash
apt-get update

apt -y install   autoconf-archive   libcmocka0   libcmocka-dev   procps   iproute2   build-essential   git   pkg-config   gcc   libtool   automake   libssl-dev   uthash-dev   autoconf   doxygen  libcurl4-openssl-dev dbus-x11 libglib2.0-dev
```

```bash
cd
git clone https://github.com/tpm2-software/tpm2-tss.git
  cd tpm2-tss
  git fetch && git fetch --tags && git checkout 2.3.1
  ./bootstrap
  ./configure --with-udevrulesdir=/etc/udev/rules.d
  make -j$(nproc)
  make install
  udevadm control --reload-rules && sudo udevadm trigger
  ldconfig
```

```bash
sudo useradd --system --user-group tss
cd
git clone https://github.com/tpm2-software/tpm2-abrmd.git
  cd tpm2-abrmd
  git fetch && git fetch --tags && git checkout 2.2.0
  ./bootstrap
  ./configure --enable-unit --with-dbuspolicydir=/etc/dbus-1/system.d
  dbus-launch make check
  sudo make install
```


```bash
cd
git clone https://github.com/tpm2-software/tpm2-tools.git
  cd tpm2-tools
  git fetch && git fetch --tags && git checkout 4.0-rc2
  ./bootstrap
  ./configure
  make check
  make install
```


```bash
cd
git clone https://github.com/tpm2-software/tpm2-tss-engine.git
  cd tpm2-tss-engine
  git fetch && git fetch --tags && git checkout v1.0.1
  ./bootstrap
  ./configure
  make -j$(nproc)
  sudo make install
```

Check if openssl works w/ tpm2 (optional)
```bash
$ openssl engine -t -c tpm2tss
    (tpm2tss) TPM2-TSS engine for OpenSSL
    [RSA, RAND]
        [ available ]
```

#### Clear TPM objects/sessions
```bash
        tpm2_flushcontext --loaded-session
        tpm2_flushcontext --saved-session
        tpm2_flushcontext --transient-object
```

### Appendix

#### References/Links

- [googe cloud credentials TPMTokenSource](https://github.com/salrashid123/oauth2#tpmtokensource)
- [TPM2-TSS-Engine hello world and Google Cloud Authentication](https://github.com/salrashid123/tpm2_evp_sign_decrypt)

- [https://www.scribd.com/document/398036850/2015-Book-APracticalGuideToTPM20](https://www.scribd.com/document/398036850/2015-Book-APracticalGuideToTPM20)
- [https://google.github.io/tpm-js](https://google.github.io/tpm-js)
- [https://www.tonytruong.net/how-to-use-the-tpm-to-secure-your-iot-device-data/](https://www.tonytruong.net/how-to-use-the-tpm-to-secure-your-iot-device-data/)
- [https://github.com/tpm2-software/tpm2-tools/wiki/Creating-Objects](https://github.com/tpm2-software/tpm2-tools/wiki/Creating-Objects)
- [https://dguerriblog.wordpress.com/2016/03/03/tpm2-0-and-openssl-on-linux-2/](https://dguerriblog.wordpress.com/2016/03/03/tpm2-0-and-openssl-on-linux-2/)
- [https://courses.cs.vt.edu/cs5204/fall10-kafura-BB/Papers/TPM/Intro-TPM-2.pdf](https://courses.cs.vt.edu/cs5204/fall10-kafura-BB/Papers/TPM/Intro-TPM-2.pdf)
