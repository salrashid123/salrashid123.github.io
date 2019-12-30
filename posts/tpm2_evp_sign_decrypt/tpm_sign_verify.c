

#include <openssl/conf.h>
#include <openssl/evp.h>
#include <openssl/err.h>
#include <openssl/pem.h>
#include <openssl/rand.h>
#include <openssl/rsa.h>
#include <openssl/engine.h>
#include <string.h>
#include <assert.h>

#include <tss2/tss2_mu.h>
#include <tss2/tss2_esys.h>
#include <tpm2-tss-engine.h>

// https://github.com/tpm2-software/tpm2-tss-engine
// tpm2tss-genkey -a rsa private.tss
// openssl req -new -x509 -engine tpm2tss -key private.tss -keyform engine -out public.crt  -subj "/C=SM/ST=somecountry/L=someloc/O=someorg/OU=somedept/CN=example.com"
// openssl x509 -pubkey -noout -in public.crt  > public.pem
// openssl x509 -in public.crt -text -noout

// export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu/engines-1.1/
// gcc tpm_sign_verify.c -L/usr/lib/x86_64-linux-gnu/engines-1.1/ -lcrypto -ltpm2tss -o tpm_sign_verify

// attribution: https://cis.gvsu.edu/~kalafuta/cis457/w19/labs/cryptotest.c
// and https://wiki.openssl.org/index.php/EVP_Signing_and_Verifying

typedef unsigned char byte;
#define UNUSED(x) ((void)x)
const char hn[] = "SHA256";

/* Returns 0 for success, non-0 otherwise */
int sign_it(const byte *msg, size_t mlen, byte **sig, size_t *slen, EVP_PKEY *pkey);

/* Returns 0 for success, non-0 otherwise */
int verify_it(const byte *msg, size_t mlen, const byte *sig, size_t slen, EVP_PKEY *pkey);

/* Prints a buffer to stdout. Label is optional */
void print_it(const char *label, const byte *buff, size_t len);

void handleErrors(void)
{
    ERR_print_errors_fp(stderr);
    abort();
}

int main(void)
{

    ENGINE *e;

    // Start TPM
    const char *engine_id = "tpm2tss";
    // End TPM

    // Start default
    // const char *engine_id = "rdrand";
    // End default

    ENGINE_load_builtin_engines();
    e = ENGINE_by_id(engine_id);
    if (!e)
    {
        printf("Unable to get Engine:\n");
        return -1;
    }
    if (!ENGINE_init(e))
    {
        printf("Unable to init Engine:\n");
        ENGINE_free(e);
        return -1;
    }

    ENGINE_set_default_ciphers(e);

    const char *pubfilename = "public.pem";
    const char *privfilename = "private.tss";

    OpenSSL_add_all_algorithms();

    EVP_PKEY *vkey, *skey;
    FILE *pubf = fopen(pubfilename, "rb");

    printf("Loading public key \n");
    vkey = PEM_read_PUBKEY(pubf, NULL, NULL, NULL);

    FILE *privf = fopen(privfilename, "rb");
    printf("Loading private key \n");

    // Start default
    //skey = PEM_read_PrivateKey(privf,NULL,NULL,NULL);
    // End default

    // Start TPM
    //
    TPM2_DATA *tpm2Data = NULL;
    tpm2tss_tpm2data_read(privfilename, &tpm2Data);
    printf("Loaded key uses alg-id %x\n", tpm2Data->pub.publicArea.type);

    if (tpm2Data->emptyAuth)
    {
        printf("EmptyAuth\n");
        tpm2Data->userauth.size = 0;
    }
    else
    {
        printf("Get User Auth\n");
    }

    switch (tpm2Data->pub.publicArea.type)
    {
    case TPM2_ALG_RSA:
        printf(" TPM2_ALG_RSA\n");
        skey = tpm2tss_rsa_makekey(tpm2Data);
        break;
    case TPM2_ALG_ECC:
        printf(" TPM2_ALG_ECC\n");
        skey = tpm2tss_ecc_makekey(tpm2Data);
        break;
    default:
        printf(" TPM2TSS_R_UNKNOWN_ALG\n");
    }
    if (!skey)
    {
        printf("TPM2TSS_R_CANNOT_MAKE_KEY\n");
    }
    printf("Loaded key uses private handle %x\n", tpm2Data->handle);
    //
    // End TPM

    assert(skey != NULL);
    if (skey == NULL)
        exit(1);

    assert(vkey != NULL);
    if (vkey == NULL)
        exit(1);

    const byte msg[] = "Now is the time for all good men to come to the aide of their country";
    byte *sig = NULL;
    size_t slen = 0;

    /* Using the skey or signing key */
    int rc = sign_it(msg, sizeof(msg), &sig, &slen, skey);
    assert(rc == 0);
    if (rc == 0)
    {
        printf("Created signature\n");
    }
    else
    {
        printf("Failed to create signature, return code %d\n", rc);
        exit(1); /* Should cleanup here */
    }

    print_it("Signature", sig, slen);

#if 0
    /* Tamper with signature */
    printf("Tampering with signature\n");
    sig[0] ^= 0x01;
#endif

#if 0
    /* Tamper with signature */
    printf("Tampering with signature\n");
    sig[slen - 1] ^= 0x01;
#endif

    /* Using the vkey or verifying key */
    rc = verify_it(msg, sizeof(msg), sig, slen, vkey);
    if (rc == 0)
    {
        printf("Verified signature\n");
    }
    else
    {
        printf("Failed to verify signature, return code %d\n", rc);
    }

    if (sig)
        OPENSSL_free(sig);

    if (skey)
        EVP_PKEY_free(skey);

    if (vkey)
        EVP_PKEY_free(vkey);

    ENGINE_finish(e);
    ENGINE_free(e);

    EVP_cleanup();
    ERR_free_strings();
    return 0;
}

int sign_it(const byte *msg, size_t mlen, byte **sig, size_t *slen, EVP_PKEY *pkey)
{
    /* Returned to caller */
    int result = -1;

    if (!msg || !mlen || !sig || !pkey)
    {
        assert(0);
        return -1;
    }

    if (*sig)
        OPENSSL_free(*sig);

    *sig = NULL;
    *slen = 0;

    EVP_MD_CTX *ctx = NULL;

    do
    {
        ctx = EVP_MD_CTX_create();
        assert(ctx != NULL);
        if (ctx == NULL)
        {
            printf("EVP_MD_CTX_create failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        const EVP_MD *md = EVP_get_digestbyname(hn);
        assert(md != NULL);
        if (md == NULL)
        {
            printf("EVP_get_digestbyname failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        int rc = EVP_DigestInit_ex(ctx, md, NULL);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestInit_ex failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        rc = EVP_DigestSignInit(ctx, NULL, md, NULL, pkey);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestSignInit failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        rc = EVP_DigestSignUpdate(ctx, msg, mlen);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestSignUpdate failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        size_t req = 0;
        rc = EVP_DigestSignFinal(ctx, NULL, &req);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestSignFinal failed (1), error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        assert(req > 0);
        if (!(req > 0))
        {
            printf("EVP_DigestSignFinal failed (2), error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        *sig = (byte *)OPENSSL_malloc(req);

        assert(*sig != NULL);
        if (*sig == NULL)
        {
            printf("OPENSSL_malloc failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        *slen = req;
        rc = EVP_DigestSignFinal(ctx, *sig, slen);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestSignFinal failed (3), return code %d, error 0x%lx\n", rc, ERR_get_error());
            break; /* failed */
        }

        assert(req == *slen);
        if (rc != 1)
        {
            printf("EVP_DigestSignFinal failed, mismatched signature sizes %ld, %ld", req, *slen);
            break; /* failed */
        }

        result = 0;

    } while (0);

    if (ctx)
    {
        EVP_MD_CTX_destroy(ctx);
        ctx = NULL;
    }

    return !!result;
}

int verify_it(const byte *msg, size_t mlen, const byte *sig, size_t slen, EVP_PKEY *pkey)
{
    /* Returned to caller */
    int result = -1;

    if (!msg || !mlen || !sig || !slen || !pkey)
    {
        assert(0);
        return -1;
    }

    EVP_MD_CTX *ctx = NULL;

    do
    {
        ctx = EVP_MD_CTX_create();
        assert(ctx != NULL);
        if (ctx == NULL)
        {
            printf("EVP_MD_CTX_create failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        const EVP_MD *md = EVP_get_digestbyname(hn);
        assert(md != NULL);
        if (md == NULL)
        {
            printf("EVP_get_digestbyname failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        int rc = EVP_DigestInit_ex(ctx, md, NULL);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestInit_ex failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        rc = EVP_DigestVerifyInit(ctx, NULL, md, NULL, pkey);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestVerifyInit failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        rc = EVP_DigestVerifyUpdate(ctx, msg, mlen);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestVerifyUpdate failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        /* Clear any errors for the call below */
        ERR_clear_error();

        rc = EVP_DigestVerifyFinal(ctx, sig, slen);
        assert(rc == 1);
        if (rc != 1)
        {
            printf("EVP_DigestVerifyFinal failed, error 0x%lx\n", ERR_get_error());
            break; /* failed */
        }

        result = 0;

    } while (0);

    if (ctx)
    {
        EVP_MD_CTX_destroy(ctx);
        ctx = NULL;
    }

    return !!result;
}

void print_it(const char *label, const byte *buff, size_t len)
{
    if (!buff || !len)
        return;

    if (label)
        printf("%s: ", label);

    for (size_t i = 0; i < len; ++i)
        printf("%02X", buff[i]);

    printf("\n");
}