
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

          String target_audience = "https://foo.com";

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
