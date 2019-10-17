
#include <openssl/conf.h>
#include <openssl/evp.h>
#include <openssl/err.h>
#include <openssl/pem.h>
#include <openssl/rand.h>
#include <openssl/rsa.h>
#include <openssl/engine.h>
#include <string.h>


// Start TPM
#include <tss2/tss2_mu.h>
#include <tss2/tss2_esys.h>
#include <tpm2-tss-engine.h>
// End TPM

// https://github.com/tpm2-software/tpm2-tss-engine
// tpm2tss-genkey -a rsa private.tss
// openssl req -new -x509 -engine tpm2tss -key private.tss -keyform engine -out public.crt  -subj "/C=SM/ST=somecountry/L=someloc/O=someorg/OU=somedept/CN=example.com"
// openssl x509 -pubkey -noout -in public.crt  > public.pem
// openssl x509 -in public.crt -text -noout


// export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu/engines-1.1/
// gcc tpm_encrypt_decrypt.c -L/usr/lib/x86_64-linux-gnu/engines-1.1/ -lcrypto -ltpm2tss -o tpm_encrypt_decrypt

// attribution: https://cis.gvsu.edu/~kalafuta/cis457/w19/labs/cryptotest.c

void handleErrors(void)
{
  ERR_print_errors_fp(stderr);
  abort();
}

int rsa_encrypt(unsigned char* in, size_t inlen, EVP_PKEY *key, unsigned char* out){ 
  EVP_PKEY_CTX *ctx;
  size_t outlen;
  ctx = EVP_PKEY_CTX_new(key, NULL);
  if (!ctx)
    handleErrors();
  if (EVP_PKEY_encrypt_init(ctx) <= 0)
    handleErrors();
  if (EVP_PKEY_CTX_set_rsa_padding(ctx, RSA_PKCS1_PADDING) <= 0)
    handleErrors();
  if (EVP_PKEY_encrypt(ctx, NULL, &outlen, in, inlen) <= 0)
    handleErrors();
  if (EVP_PKEY_encrypt(ctx, out, &outlen, in, inlen) <= 0)
    handleErrors();
  return outlen;
}

int rsa_decrypt(unsigned char* in, size_t inlen, EVP_PKEY *key, unsigned char* out){ 
  EVP_PKEY_CTX *ctx;
  size_t outlen;
  ctx = EVP_PKEY_CTX_new(key,NULL);
  if (!ctx)
    handleErrors();
  if (EVP_PKEY_decrypt_init(ctx) <= 0)
    handleErrors();
  if (EVP_PKEY_CTX_set_rsa_padding(ctx, RSA_PKCS1_PADDING) <= 0)
    handleErrors();
  if (EVP_PKEY_decrypt(ctx, NULL, &outlen, in, inlen) <= 0)
    handleErrors();
  if (EVP_PKEY_decrypt(ctx, out, &outlen, in, inlen) <= 0)
    handleErrors();
  return outlen;
}

int encrypt(unsigned char *plaintext, int plaintext_len, unsigned char *key,
	unsigned char *iv, unsigned char *ciphertext){
  EVP_CIPHER_CTX *ctx;
  int len;
  int ciphertext_len;
  if(!(ctx = EVP_CIPHER_CTX_new())) handleErrors();
  if(1 != EVP_EncryptInit_ex(ctx, EVP_aes_256_cbc(), NULL, key, iv))
    handleErrors();
  if(1 != EVP_EncryptUpdate(ctx, ciphertext, &len, plaintext, plaintext_len))
    handleErrors();
  ciphertext_len = len;
  if(1 != EVP_EncryptFinal_ex(ctx, ciphertext + len, &len)) handleErrors();
  ciphertext_len += len;
  EVP_CIPHER_CTX_free(ctx);
  return ciphertext_len;
}

int decrypt(unsigned char *ciphertext, int ciphertext_len, unsigned char *key,
	    unsigned char *iv, unsigned char *plaintext){
  EVP_CIPHER_CTX *ctx;
  int len;
  int plaintext_len;
  if(!(ctx = EVP_CIPHER_CTX_new())) handleErrors();
  if(1 != EVP_DecryptInit_ex(ctx, EVP_aes_256_cbc(), NULL, key, iv))
    handleErrors();
  if(1 != EVP_DecryptUpdate(ctx, plaintext, &len, ciphertext, ciphertext_len))
    handleErrors();
  plaintext_len = len;
  if(1 != EVP_DecryptFinal_ex(ctx, plaintext + len, &len)) handleErrors();
  plaintext_len += len;
  EVP_CIPHER_CTX_free(ctx);
  return plaintext_len;
}

int main(void){

 ENGINE *e;

 // Start TPM
 const char *engine_id = "tpm2tss";
 // End TPM

 // Start default
 //const char *engine_id = "rdrand";
 // End default

 ENGINE_load_builtin_engines();
 e = ENGINE_by_id(engine_id);
 if (!e) {
     printf("Unable to get Engine:\n");
     return -1;
 }
 if (!ENGINE_init(e)) {
     printf("Unable to init Engine:\n");
     ENGINE_free(e);
     return -1;
 }
 if (!ENGINE_set_default_RSA(e))
     abort();

 ENGINE_set_default_ciphers(e);

  const  char* pubfilename =  "public.pem";
  const  char* privfilename = "private.tss";


  unsigned char key[32];
  unsigned char iv[16];
  unsigned char *plaintext =
  (unsigned char *)"This is a test string to encrypt.";
  unsigned char ciphertext[1024];
  unsigned char decryptedtext[1024];
  int decryptedtext_len, ciphertext_len;

  OpenSSL_add_all_algorithms();
  RAND_bytes(key,32);
  RAND_bytes(iv,16);
  EVP_PKEY *pubkey, *privkey;
  FILE* pubf = fopen(pubfilename,"rb");
  pubkey = PEM_read_PUBKEY(pubf,NULL,NULL,NULL);
  unsigned char encrypted_key[256];
  int encryptedkey_len = rsa_encrypt(key, 32, pubkey, encrypted_key);
  ciphertext_len = encrypt (plaintext, strlen ((char *)plaintext), key, iv,
                            ciphertext);
  printf("Ciphertext is:\n");
  EVP_PKEY_free(pubkey);

  BIO_dump_fp (stdout, (const char *)ciphertext, ciphertext_len);

  FILE* privf = fopen(privfilename,"rb");

  printf("Loading private key \n");

  // Start default
  // privkey = PEM_read_PrivateKey(privf,NULL,NULL,NULL);
  // End default

  // Start TPM >>>>> 
  //
  TPM2_DATA *tpm2Data = NULL;
  tpm2tss_tpm2data_read(privfilename, &tpm2Data);
  printf("Loaded key uses alg-id %x\n", tpm2Data->pub.publicArea.type);

  if (tpm2Data->emptyAuth) {
      printf("EmptyAuth\n");
      tpm2Data->userauth.size = 0;
  } else {
      printf("Get User Auth\n");
  }

  switch (tpm2Data->pub.publicArea.type) {
  case TPM2_ALG_RSA:
      printf(" TPM2_ALG_RSA\n");
      privkey = tpm2tss_rsa_makekey(tpm2Data);
      break;
  case TPM2_ALG_ECC:
      printf(" TPM2_ALG_ECC\n");
      privkey = tpm2tss_ecc_makekey(tpm2Data);
      break;
  default:
      printf(" TPM2TSS_R_UNKNOWN_ALG\n");
  }
  if (!privkey) {
      printf("TPM2TSS_R_CANNOT_MAKE_KEY\n");
  }

  printf("Loaded key uses private handle %x\n", tpm2Data->handle);
  
  // <<< END TPM

  unsigned char decrypted_key[32];
  int decryptedkey_len = rsa_decrypt(encrypted_key, encryptedkey_len, privkey, decrypted_key); 
  decryptedtext_len = decrypt(ciphertext, ciphertext_len, decrypted_key, iv, decryptedtext);
  decryptedtext[decryptedtext_len] = '\0';
  printf("Decrypted text is:\n");
  printf("%s\n", decryptedtext);

  EVP_PKEY_free(privkey);

  ENGINE_finish(e);
  ENGINE_free(e);

  EVP_cleanup();
  ERR_free_strings();
  return 0;
}




