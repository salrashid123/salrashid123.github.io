# Authenticating using Google OpenID Connect Tokens


* [What is an id_token?](#what-is-an-id-token)
* [Whats an Audience?](#what-is-an-audience)
* [Sources of Google Issued ID Tokens](#sources-of-google-issued-id-tokens)
* [Making Authorized Requests](#making-authorized-requests)
* [Services Accepting OIDC tokens for authentication](#services-accepting-oidc-tokens-for-authentication)
* [Services that include OIDC tokens in webhooks](#services-that-include-oidc-tokens-in-webhooks)
* [How to get an ID Token?](#how-to-get-an-id-token)
  * [gcloud](#gcloud)
  * [python](#python)
  * [java](#java)
  * [go](#go)
  * [nodejs](#nodejs)
  * [dotnet](#dotnet)
* [How to verify an ID Token?](#how-to-verify-an-id-token)
* [References](#references)



This section covers authenticating against security perimeters which requires clients present valid [OpenID Connect tokens](https://openid.net/specs/openid-connect-core-1_0.html#IDToken). These security perimeters do not protect Google APIs but your services deployed behind certain Google Cloud Products. For example, if you deploy to [Cloud Functions](https://cloud.google.com/functions/docs/) or an application on [Cloud Run](https://cloud.google.com/run/docs/), you can enable a perimeter such that any client that wants to invoke the function or your application must present an ID token issued by Google. 

These tokens are not Oauth2 [access_tokens](https://developers.google.com/identity/protocols/OAuth2) you would use to call a Google Service or API directly such as a Google Compute Engine API or Cloud Storage Bucket but id_tokens that assert identity and are signed by Google.

## What is an id_token?

OpenIDConnect (OIDC) tokens are signed JSON Web Tokens [JWT](https://tools.ietf.org/html/rfc7519) used to assert identity and do not necessarily carry any implicit authorization against a resource. These tokens will just declare who the caller is and any service that the token is sent to can verify the token's integrity by verifying the signature payload provided with the JWT.  For more information, see the links in the [References](#references) section below

If the ID Token is signed and issued by Google, that token can be used as a token against GCP service perimeters because the service can decode the token, verify its signature and finally identify the caller using values within the JWT claim. For example, the JWT header and payload below describes a token that was issued by google (`"iss": "https://accounts.google.com"`), identifies the caller (`"email": "svc_account@.project.gserviceaccount.com"`), has not expired (the service will check the `exp:` field), and finally will verify the JWT is intended for the service or not to `"aud": "https://example.com"`.


```json
    {
    "alg": "RS256",
    "kid": "5d887f26ce32577c4b5a8a1e1a52e19d301f8181",
    "typ": "JWT"
    }.
    {
    "aud": "https://example.com",
    "azp": "107145139691231222712",
    "email": "svc_account@.project.gserviceaccount.com",
    "email_verified": true,
    "exp": 1556665461,
    "iat": 1556661861,
    "iss": "https://accounts.google.com",
    "sub": "107145139691231222712"
    }
```

>> Note:  the sub claim in the token above represents the unique internal Google identifier account representing the ID Token.

## Whats an Audience?

The `aud:` field describes the service name this token was created to invoke. If a service receives an id_token, it must verify its integrity (signature), validity (is it expired) and if the aud: field is the predefined name it expects to see. If the names do not match, the service should reject the token as it could be a replay intended for another system.

Both Google [Service Accounts](https://cloud.google.com/iam/docs/service-accounts) and Users can get id_tokens but with an important distinction: User login oauth flows issue id_tokens statically bound to the web or oauth2 client_id the flow as associated with. That is, if a user logs into a web application involving oauth2, the id_token that the provider issues to the browser will have the aud: field bound to the oauth2 client_id.

Service Accounts on the other hand, can participate in a flow where it can receive and id_token from google with an aud: field it specified earlier. These token types issued by Service Accounts are generally the ones discussed in this article.

## Sources of Google Issued ID Tokens

There are several ways to get a Google-issued id_token for a Service Account

### Service Account JSON certificate

If you have a Google-issued Service account certificate file locally, you can sign the JWT with specific claims and exchange that with google to get a google-issued id_token. While specifying the claims to sign, a predefined claim called target_audience which when set will be used by Google oauth endpoint and reinterpreted as the aud: field in the id_token.

The flow using a json is:
  *  Use the service account JSON file to sign a JWT with intended final audience set as target_audience.
  *  Exchange the signed JWT with Google token endpoint: https://www.googleapis.com/oauth2/v3/certs
  *  Google will verify the signature and identify the aller as the Service Account (since the caller had possession of the private key), then issue an id_token with the aud: field set to what the target_audience was set.
  *  Return the id_token in the response back to the client.

### Metadata Server

If a metadata server is available while running on Compute Engine, Appengine 2nd Generation, Cloud Functions or even Kubernetes engine, getting an id_token is simple: query the server itself for the token and provide the audience field the token should be for.

For example, the following `curl` command on any platform with metadata server will return an id_token:

```bash
curl -s-H 'Metadata-Flavor: Google' http://metadata/computeMetadata/v1/instance/service-accounts/default/identity?audience=https://example.com`
```

### IAMCredentials generateIdToken()

Google [Cloud IAM Credentials API](https://cloud.google.com/iam/credentials/reference/rest/) provides a way for one service account to generate short lived tokens on behalf of another. One of the token types it can issue is an `id_token` via the [generateIdToken()](https://cloud.google.com/iam/credentials/reference/rest/v1/projects.serviceAccounts/generateIdToken) endpoint.
Making Authorized Requests
Once you have an id_token, provide that in the request Authorization header as:

```
Authorization: Bearer id_token
```

eg.
```bash
curl -v -H "Authorization: Bearer id_token" https://some-cloud-run-uc.a.run.app
```

## Services Accepting OIDC tokens for authentication

The following platforms use Google OIDC tokens for access controls. If you deploy an application behind any of of these services, you can optionally enable IAM access controls. What that will do is require any inbound access to a service to provide a valid Google OIDC token.

Furthermore, the token must have its aud: field set to the service name being invoked. For example, to invoke a Cloud Run service, you must setup IAM access for the users (see [Managing access via IAM](https://cloud.google.com/run/docs/reference/iam/roles) and any ID token provided must have be signed with the aud: field set to the service name itself. If the Cloud Run service is https://svc.-hash-.zone.cloud.run, the audience field must be set to the same

* [Google Cloud Run](https://cloud.google.com/run/)
* [Google Cloud Functions](https://cloud.google.com/functions/docs/)
* [Programmatic authentication Google Identity Aware Proxy](https://cloud.google.com/iap/docs/authentication-howto)
* [Google Cloud Endpoints](https://cloud.google.com/endpoints/docs/openapi/authenticating-users-google-id) (if using Google OIDC)

You can also deploy your own service outside of these services and verifying an OpenID Connect token. In this mode, your application that receives an OIDC token will need to manually verify its validity and audience field. You can use application frameworks like to do this like Spring Security, proxies like Envoy or even higher level Services like Istio.

For detailed implementation, see:
- [Automatic OIDC: Using Cloud Scheduler, Tasks, and PubSub to make authenticated calls to Cloud Run, Cloud Functions or your Server](https://medium.com/google-cloud/automatic-oidc-using-cloud-scheduler-tasks-and-pubsub-to-make-authenticated-calls-to-cloud-run-de9e7e9cec3f)

## Services that include OIDC tokens in webhooks

Other services also support automatically including an OIDC token along with a webhook request

* [Cloud Tasks](https://cloud.google.com/tasks/)
* [Cloud Scheduler](https://cloud.google.com/scheduler/)
* [Cloud Pub/Sub](https://cloud.google.com/pubsub/docs/)

For example, you can configure Cloud Scheduler to emit an OIDC token with a preset audience. When a scheduled tasks fires, an http webhook url will be called and within the header payload, the OIDC token will get transmitted within the `Authorization` header. The webhook target can be your own application or any of the services listed in the previous section. If your application is running outside of these services listed under `Services Accepting OIDC tokens` for authentication, you will need to parse and verify the OIDC token.

See:

* [Cloud Scheduler OIDC](https://cloud.google.com/scheduler/docs/http-target-auth#token)
* [Cloud Pub/Sub OIDC](https://cloud.google.com/pubsub/docs/push#using_json_web_tokens_jwts)
* [Cloud Tasks OIDC](https://cloud.google.com/tasks/docs/reference/rpc/google.cloud.tasks.v2beta3#oidctoken)

For detailed implementation, see:
- [Automatic oauth2: Using Cloud Scheduler and Tasks to call Google APIs](https://medium.com/google-cloud/automatic-oauth2-using-cloud-scheduler-and-tasks-to-call-google-apis-55f0a8905baf)


## How to get an ID Token

There are several flows to get an ID Token available. The snippets below demonstrate how to

1. Get an IDToken
2. Verify an IDToken
3. Issue an authenticated request using the IDToken

Each while using

* Service Account JSON certificate
* Compute Engine Credentials

### gcloud

```bash
 gcloud auth activate-service-account --key-file=/path/to/svc_account.json
 gcloud auth print-identity-token --audience=https://example.com
```

* 7/10/19: gcloud supports only getting an IDToken for Service Account JSON

### python

```python
import google.oauth2.credentials
from google.oauth2 import id_token
from google.oauth2 import service_account
import google.auth
import google.auth.transport.requests
from google.auth.transport.requests import AuthorizedSession

# pip install google-auth requests

target_audience = 'https://example.com'

url = 'https://example.com'
certs_url='https://www.googleapis.com/oauth2/v1/certs'
metadata_identity_doc_url = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity"

svcAccountFile = '/path/to/svc_account.json'

def GetIDTokenFromServiceAccount(svcAccountFile, target_audience):
  creds = service_account.IDTokenCredentials.from_service_account_file(
        svcAccountFile,
        target_audience= target_audience)
  request = google.auth.transport.requests.Request()
  creds.refresh(request)
  return creds.token

def GetIDTokenFromComputeEngine(target_audience):
  request = google.auth.transport.requests.Request()
  url = metadata_identity_doc_url + "?audience=" + target_audience
  headers = {"Metadata-Flavor":"Google" }
  resp = request(url, method='GET', headers=headers)
  return resp.data

def VerifyIDToken(token, certs_url,  audience=None):
   request = google.auth.transport.requests.Request()
   result = id_token.verify_token(token,request,certs_url=certs_url)
   if audience in result['aud']:
     return True
   return False

def MakeAuthenticatedRequest(id_token, url):
  creds = google.oauth2.credentials.Credentials(id_token)
  authed_session = AuthorizedSession(creds)
  r = authed_session.get(url)
  print r.status_code
  print r.text

# For ServiceAccount
token = GetIDTokenFromServiceAccount(svcAccountFile,target_audience)

# For Compute Engine
#token = GetIDTokenFromComputeEngine(target_audience)

print 'Token: ' + token
if VerifyIDToken(token=token,certs_url=certs_url, audience=target_audience):
  print 'token Verified with aud: ' + target_audience
print 'Making Authenticated API call:'
MakeAuthenticatedRequest(token,url)
```

### java

```java

package com.test;

import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.URL;
import java.text.ParseException;
import java.time.Clock;
import java.time.Instant;
import java.util.Collections;
import java.util.Date;
import java.util.List;

import com.google.api.client.googleapis.auth.oauth2.GoogleIdToken;
import com.google.api.client.googleapis.auth.oauth2.GoogleIdToken.Payload;
import com.google.api.client.googleapis.auth.oauth2.GoogleIdTokenVerifier;
import com.google.api.client.http.GenericUrl;
import com.google.api.client.http.HttpHeaders;
import com.google.api.client.http.HttpRequest;
import com.google.api.client.http.HttpRequestFactory;
import com.google.api.client.http.HttpResponse;
import com.google.api.client.http.HttpStatusCodes;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.UrlEncodedContent;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.JsonObjectParser;
import com.google.api.client.json.jackson2.JacksonFactory;
import com.google.api.client.util.GenericData;
import com.google.auth.oauth2.ServiceAccountCredentials;
import com.nimbusds.jose.JOSEException;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JWSSigner;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.source.JWKSource;
import com.nimbusds.jose.jwk.source.RemoteJWKSet;
import com.nimbusds.jose.proc.JWSKeySelector;
import com.nimbusds.jose.proc.JWSVerificationKeySelector;
import com.nimbusds.jose.proc.SecurityContext;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import com.nimbusds.jwt.proc.ConfigurableJWTProcessor;
import com.nimbusds.jwt.proc.DefaultJWTProcessor;


/* pom.xml

<dependency>
  <groupId>com.google.auth</groupId>
  <artifactId>google-auth-library-oauth2-http</artifactId>
  <version>0.15.0</version>
</dependency>

<dependency>
    <groupId>com.nimbusds</groupId>
    <artifactId>nimbus-jose-jwt</artifactId>
    <version>7.2.1</version>
</dependency>

<dependency>
    <groupId>com.google.api-client</groupId>
    <artifactId>google-api-client</artifactId>
    <version>1.30.2</version>
</dependency>
*/

public class GoogleIDToken {

     private String id_token;
     private Date expiry;

     private static final String OAUTH_TOKEN_URI = "https://www.googleapis.com/oauth2/v4/token";
     private static final String GOOGLE_CERT_URL = "https://www.googleapis.com/oauth2/v3/certs";
     private static final String JWT_BEARER_TOKEN_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:jwt-bearer";
     private static final long EXPIRATION_TIME_IN_SECONDS = 3600L;
     private static final String METADATA_IDENTITY_DOC_URL = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity";

     public static void main(String[] args) throws Exception {

          String target_audience = "https://cloud-runapp.run.app";

          GoogleIDToken tc = new GoogleIDToken();

          String credFile = "/path/to/svc.json";
          ServiceAccountCredentials sac = ServiceAccountCredentials.fromStream(new FileInputStream(credFile));

          String tok = tc.getIDTokenFromServiceAccount(sac, target_audience);
          System.out.println(tok);
          System.out.println(GoogleIDToken.verifyToken(tok, target_audience));

          // tok = tc.getIDTokenFromComputeEngine(target_audience);
          // System.out.println(tok);
          // System.out.println(GoogleIDToken.verifyToken(tok, target_audience);

          String url = "https://cloud-runapp.run.app";
          tc.MakeAuthenticatedRequest(tok, url);
     }

     public String getIDToken() {
          return this.id_token;
     }

     public Date getExpiry() {
          // check if expired..
          return this.expiry;
     }

     public String getIDTokenFromServiceAccount(ServiceAccountCredentials sc, String audience) throws Exception {

          Clock c = Clock.systemUTC();
          Instant now = Instant.now(c);
          long expirationTime = now.getEpochSecond() + EXPIRATION_TIME_IN_SECONDS;

          JWSHeader jwsHeader = new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(sc.getPrivateKeyId()).build();

          JWTClaimsSet claims = new JWTClaimsSet.Builder().audience(OAUTH_TOKEN_URI).issuer(sc.getClientEmail())
                    .subject(sc.getClientEmail()).issueTime(Date.from(now))
                    .expirationTime(Date.from(Instant.ofEpochSecond(expirationTime))).claim("target_audience", audience)
                    .build();

          JWSSigner signer = new RSASSASigner(sc.getPrivateKey());
          SignedJWT signedJwt = new SignedJWT(jwsHeader, claims);

          try {
               signedJwt.sign(signer);
          } catch (JOSEException jse) {
               throw new Exception(jse.getMessage());
          }

          HttpTransport httpTransport = new NetHttpTransport();
          GenericData tokenRequest = new GenericData().set("grant_type", JWT_BEARER_TOKEN_GRANT_TYPE).set("assertion",
                    signedJwt.serialize());
          UrlEncodedContent content = new UrlEncodedContent(tokenRequest);

          HttpRequestFactory requestFactory = httpTransport.createRequestFactory();
          HttpRequest request = requestFactory.buildPostRequest(new GenericUrl(OAUTH_TOKEN_URI), content)
                    .setParser(new JsonObjectParser(JacksonFactory.getDefaultInstance()));

          HttpResponse response = request.execute();
          int statusCode = response.getStatusCode();
          if (statusCode != HttpStatusCodes.STATUS_CODE_OK) {
               throw new IOException(
                         String.format("Unable to get IDToken %s: %s", statusCode, response.parseAsString()));
          }
          GenericData responseData = response.parseAs(GenericData.class);
          id_token = (String) responseData.get("id_token");
          try {
               SignedJWT sw = SignedJWT.parse(id_token);
               this.id_token = sw.getParsedString();
               this.expiry = sw.getJWTClaimsSet().getExpirationTime();
          } catch (ParseException jse) {
               throw new Exception(jse.getMessage());
          }
          return this.id_token;
     }

     public String getIDTokenFromComputeEngine(String audience) throws Exception {

          GenericUrl tokenUrl = new GenericUrl(METADATA_IDENTITY_DOC_URL);
          tokenUrl.put("audience", audience);

          HttpTransport httpTransport = new NetHttpTransport();
          HttpRequestFactory requestFactory = httpTransport.createRequestFactory();
          HttpRequest request = requestFactory.buildGetRequest(tokenUrl);
          HttpHeaders headers = new HttpHeaders();
          headers.put("Metadata-Flavor", "Google");
          request.setHeaders(headers);
          HttpResponse response = request.execute();
          int statusCode = response.getStatusCode();
          if (statusCode != HttpStatusCodes.STATUS_CODE_OK) {
               throw new IOException(String.format("Unable to get IDToken from metadataServer %s: %s", statusCode,
                         response.parseAsString()));
          }
          InputStream is = response.getContent();
          StringBuilder sb = new StringBuilder();
          int ch;
          while ((ch = is.read()) != -1) {
               sb.append((char) ch);
          }
          response.disconnect();
          id_token = sb.toString();

          try {
               SignedJWT sw = SignedJWT.parse(id_token);
               this.id_token = sw.getParsedString();
               this.expiry = sw.getJWTClaimsSet().getExpirationTime();
          } catch (ParseException jse) {
               throw new Exception(jse.getMessage());
          }
          return this.id_token;
     }

     public static boolean verifyTokenWithNimbus(String id_token, String audience) throws Exception {
          JWKSource keySource = new RemoteJWKSet(new URL(GOOGLE_CERT_URL));
          ConfigurableJWTProcessor jwtProcessor = new DefaultJWTProcessor();
          JWSAlgorithm expectedJWSAlg = JWSAlgorithm.RS256;
          JWSKeySelector keySelector = new JWSVerificationKeySelector(expectedJWSAlg, keySource);
          jwtProcessor.setJWSKeySelector(keySelector);

          SecurityContext ctx = null;

          SignedJWT jwtToken = SignedJWT.parse(id_token);

          boolean isValidAudience = false;
          List<String> tokenAudienceList = jwtToken.getJWTClaimsSet().getAudience();
          for (String aud : tokenAudienceList) {
               if (audience.contains(aud)) {
                    isValidAudience = true;
                    break;
               }
          }
          if (!isValidAudience)
               return false;
          JWTClaimsSet claimsSet = jwtProcessor.process(id_token, ctx);
          return true;
     }

     public static boolean verifyToken(String id_token, String audience) throws Exception {

          JacksonFactory jacksonFactory = new JacksonFactory();
          GoogleIdTokenVerifier verifier = new GoogleIdTokenVerifier.Builder(new NetHttpTransport(), jacksonFactory)
                    .setAudience(Collections.singletonList(audience)).build();

          GoogleIdToken idToken = verifier.verify(id_token);
          if (idToken != null) {
               Payload payload = idToken.getPayload();
               // System.out.println(payload);
               return true;
          } else {
               return false;
          }
     }

     public void MakeAuthenticatedRequest(String id_token, String url) throws IOException {
          GenericUrl tokenUrl = new GenericUrl(url);

          HttpTransport httpTransport = new NetHttpTransport();
          HttpRequestFactory requestFactory = httpTransport.createRequestFactory();
          HttpRequest request = requestFactory.buildGetRequest(tokenUrl);
          HttpHeaders headers = new HttpHeaders();
          headers.put("Authorization", "Bearer " + id_token);
          request.setHeaders(headers);
          HttpResponse response = request.execute();
          InputStream is = response.getContent();
          StringBuilder sb = new StringBuilder();
          int ch;
          while ((ch = is.read()) != -1) {
               sb.append((char) ch);
          }
          response.disconnect();
          System.out.println("Response from Cloud Run: " + sb.toString());
     }

}
```

### go

```golang
package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"time"

	"golang.org/x/net/context"

	"golang.org/x/oauth2/google"

	"golang.org/x/oauth2/jws"
	"github.com/coreos/go-oidc"
)

const (
	googleRootCertURL = "https://www.googleapis.com/oauth2/v3/certs"
	audience = "https://example.com"
	jsonCert            = "/path/to/service_account.json"
	metadataIdentityDocURL = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity"
)

func getIDTokenFromServiceAccount(ctx context.Context, audience string) (string, error) {
	data, err := ioutil.ReadFile(jsonCert)
	if err != nil {
		return "", err
	}

	conf, err := google.JWTConfigFromJSON(data, "")
	if err != nil {
		return "", err
	}

	header := &jws.Header{
		Algorithm: "RS256",
		Typ:       "JWT",
		KeyID:     conf.PrivateKeyID,
	}

	privateClaims := map[string]interface{}{"target_audience": audience }
	iat := time.Now()
	exp := iat.Add(time.Hour)

	payload := &jws.ClaimSet{
		Iss:           conf.Email,
		Iat:           iat.Unix(),
		Exp:           exp.Unix(),
		Aud:           "https://www.googleapis.com/oauth2/v4/token",
		PrivateClaims: privateClaims,
	}

	key := conf.PrivateKey
	block, _ := pem.Decode(key)
	if block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return "", err
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		log.Fatal("private key is invalid")
	}

	token, err := jws.Encode(header, payload, parsed)
	if err != nil {
		return "", err
	}

	d := url.Values{}
	d.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	d.Add("assertion", token)

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://www.googleapis.com/oauth2/v4/token", strings.NewReader(d.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var y map[string]interface{}
	err = json.Unmarshal([]byte(body), &y)
	if err != nil {
		return "", err
	}
	return y["id_token"].(string), nil
}

func getIDTokenFromComputeEngine(ctx context.Context, audience string) (string, error) {
    client := &http.Client{}
    req, err := http.NewRequest("GET", metadataIdentityDocURL + "?audience=" + audience, nil)
    req.Header.Add("Metadata-Flavor", "Google")
    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }

    bodyString := string(bodyBytes)
    return bodyString, nil
}
func verifyGoogleIDToken(ctx context.Context, aud string, token string) (bool, error) {

	keySet := oidc.NewRemoteKeySet(ctx, googleRootCertURL)

	var config = &oidc.Config{
		SkipClientIDCheck: true,
		ClientID: aud,
	}
	verifier := oidc.NewVerifier("https://accounts.google.com", keySet, config)
	
	idt, err := verifier.Verify(ctx, token)
	if err != nil {
		return false, err
	}
	log.Printf("Verified id_token with Issuer %v: ", idt.Issuer)
	return true, nil
}

func makeAuthenticatedRequest(idToken string, url string) {

    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    req.Header.Add("Authorization", "Bearer " + idToken)
    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }

    bodyString := string(bodyBytes)
    log.Printf("Authenticated Response: %v",bodyString)
}


func main() {

	ctx := context.Background()
	
	// For Service Account
	idToken, err := getIDTokenFromServiceAccount(ctx, audience)
	
	// For Compute Engine
	//idToken, err := getIDTokenFromComputeEngine(ctx,audience)
	
	if err != nil {
		log.Fatalf("%v",err)
	}

	log.Printf("IDToken: %v", idToken)
	verified, err := verifyGoogleIDToken(ctx, audience, idToken)
	if err != nil {
		log.Fatalf("%v",err)
	}
	log.Printf("Verify %v", verified)

	u := "https://example.com"
	makeAuthenticatedRequest(idToken, u)

}

```
### nodejs

```javascript
const {JWT} = require('google-auth-library');
const {OAuth2Client} = require('google-auth-library');
const fetch = require("node-fetch");

const audience = 'https://example.com';
const url = 'https://example.com';

const certs_url='https://www.googleapis.com/oauth2/v1/certs'
const metadata_identity_doc_url = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity"

async function getIDTokenFromComputeEngine(audience) {
  return fetch(metadata_identity_doc_url + '?audience=' + audience,  { headers: {'Metadata-Flavor': 'Google',} })
  .then(res => res.text())
  .then(body => { return body });
}


async function getIDTokenFromServiceAccount(audience) {
  const keys = require('/path/to/svc.json');
  const opts = {
    "email": keys.client_email,
    "key": keys.private_key,
    "additionalClaims": {"target_audience": audience}
  }
  const client = new JWT(opts);

  const tokenInfo = await client.authorizeAsync();
  
  return tokenInfo.id_token
}

async function verifyIDToken(token, audience, url) {
  const oAuth2Client = new OAuth2Client(audience);
  const ticket = await oAuth2Client.verifyIdToken({
    idToken: token,
    audience: audience,
  });
  return true;
}

async function main() {
  // If ComputeEngine
  //const id_token = await getIDTokenFromServiceAccount(audience);
  
  // If Service Account
  const id_token = await getIDTokenFromComputeEngine(audience);
  
  console.log(id_token);

  let validated = await verifyIDToken(id_token,audience,certs_url);
  if (validated) {
    console.log("id_token validated with audience " + audience);
  }


}

main().catch(console.error);
```

### dotnet

```c#
using System;
using System.IO;
using System.Text;
using System.Threading.Tasks;

using Google.Apis.Auth.OAuth2;
using Google.Apis.Auth;
using Newtonsoft.Json.Linq;
using System.Net;
using System.Collections.Specialized;
using Microsoft.AspNetCore.WebUtilities;

namespace GoogleIDToken
{

    public class GoogleIDToken 
    {
        [STAThread]
        static void Main(string[] args)
        {

          var target_audience = "https://example.com";
          
          string CREDENTIAL_FILE_JSON = "/path/to/service_account.json";
          ServiceAccountCredential svc_credential;

          using (var stream = new FileStream(CREDENTIAL_FILE_JSON, FileMode.Open, FileAccess.Read))
          {
              svc_credential = ServiceAccountCredential.FromServiceAccountData(stream);
          }            
          GoogleIDToken  gid = new GoogleIDToken();
            
          // For ServiceAccount
          string id_token = gid.GetIDTokenFromServiceAccount(svc_credential, target_audience);
          // For Compute Engine
          //string id_token = gid.GetIDTokenFromComputeEngine(target_audience); 
            
          Console.WriteLine("ID Token: " + id_token);
          gid.VerifyIDToken(id_token).Wait();

          string url = "https://example.com";
          gid.MakeAuthenticatedRequest(id_token,url);
        }

        private string GetIDTokenFromServiceAccount(ServiceAccountCredential svc_credential, string target_audience)
        {  
            TimeSpan unix_time = (System.DateTime.UtcNow - new DateTime(1970, 1, 1, 0, 0, 0));

            long now = (long)unix_time.TotalSeconds;
            long exp = now + 3600;
            JObject header = new JObject
            {
                ["alg"] = "RS256",
                ["type"] = "JWT"
            };
            
            JObject claim = new JObject {
                ["iss"] = svc_credential.Id,
                ["aud"] = "https://oauth2.googleapis.com/token",
                ["exp"] =  exp,
                ["iat"] =  now,
                ["target_audience"] = target_audience 
            };
                
            System.Text.UTF8Encoding e = new System.Text.UTF8Encoding();
            String jwt_header_and_claim = WebEncoders.Base64UrlEncode(e.GetBytes(header.ToString())) + "." +  WebEncoders.Base64UrlEncode(e.GetBytes(claim.ToString()));            

            var jwt_signature = svc_credential.CreateSignature(System.Text.Encoding.UTF8.GetBytes(jwt_header_and_claim)); 

            WebClient client = new WebClient();
            NameValueCollection formData = new NameValueCollection();
            formData["grant_type"] = "urn:ietf:params:oauth:grant-type:jwt-bearer";
            formData["assertion"] = jwt_header_and_claim + "." + jwt_signature;;
            client.Headers["Content-type"] = "application/x-www-form-urlencoded";
            try
                {
                    byte[] responseBytes = client.UploadValues("https://oauth2.googleapis.com/token", "POST", formData);
                    string Result = Encoding.UTF8.GetString(responseBytes);
    
                    var assertion_response = JObject.Parse(Result); 
                    var id_token = assertion_response["id_token"];

                    return id_token.ToString();

                } catch (WebException ex)
                {
                    Stream receiveStream = ex.Response.GetResponseStream();
                    Encoding encode = System.Text.Encoding.GetEncoding("utf-8");
                    StreamReader readStream = new StreamReader(receiveStream, encode);
                    string pageContent = readStream.ReadToEnd();
                    Console.WriteLine("Error: " + pageContent);
                    throw new System.Exception("Error while getting IDToken " + ex.Message);
                }                 
        }      
            
        private async Task VerifyIDToken(string id_token)
        {        
                
          // Verify Token
          var validPayload = await GoogleJsonWebSignature.ValidateAsync(id_token.ToString(),null,true);
          double timestamp = Convert.ToDouble(validPayload.ExpirationTimeSeconds);
          System.DateTime dateTime = new System.DateTime(1970, 1, 1, 0, 0, 0, 0);
          dateTime = dateTime.AddSeconds(timestamp);             
          Console.WriteLine("Token Verified; Expires at : " + dateTime);
        }

        private string GetIDTokenFromComputeEngine(string target_audience)
        {        

            string metadata_url = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity?audience=" + target_audience;
            var client = new WebClient();            
            client.Headers.Add("Metadata-Flavor", "Google");
            var id_token = client.DownloadString(metadata_url);
            return id_token;
        }


        private void MakeAuthenticatedRequest(string id_token, string url) {
            var client = new WebClient();            
            client.Headers.Add("Authorization", "Bearer " + id_token);
            var result = client.DownloadString(url);
            Console.WriteLine(result);
        }
    }
}
```
## How to verify an ID Token?

You can verify OIDC tokens manually if the inbound framework you deployed an application to does automatically perform the validation. In these cases, you the snippets provided above describe how to use google and other libraries to download the public certificates used to sign the JWT

* Google Public Certificate URL [https://www.googleapis.com/oauth2/v3/certs](https://www.googleapis.com/oauth2/v3/certs)

Any validation should not just involve verifying the public certificate and validity but also that the audience claim matches the service being invoked.  For more information, see [Validating an ID Token](https://developers.google.com/identity/protocols/OpenIDConnect#validatinganidtoken).  You can find samples [here](https://developers.google.com/identity/sign-in/web/backend-auth#verify-the-integrity-of-the-id-token) that implement validation.

It is recommend to always verify locally but for debugging, you can use the [tokenInfo endpoint](https://developers.google.com/identity/sign-in/web/backend-auth#calling-the-tokeninfo-endpoint) or services that decode like jwt.io.

## References

* [ID Tokens Explained](https://www.oauth.com/oauth2-servers/openid-connect/id-tokens/)
* [OpenID Connect id_token](https://openid.net/specs/openid-connect-core-1_0.html#CodeIDToken)
* [OAuth2 access_token](https://developers.google.com/identity/protocols/OAuth2)
* [OpenID Connect on Google APIs](https://developers.google.com/identity/protocols/OpenIDConnect)