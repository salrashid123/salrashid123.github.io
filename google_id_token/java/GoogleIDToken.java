/*
 * Copyright 2019 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.test;

import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.util.Collections;
import java.util.Date;
import java.util.List;
import java.util.Map;

import com.google.api.client.googleapis.auth.oauth2.GoogleIdToken;
import com.google.api.client.googleapis.auth.oauth2.GoogleIdToken.Payload;
import com.google.api.client.googleapis.auth.oauth2.GoogleIdTokenVerifier;
import com.google.api.client.http.GenericUrl;
import com.google.api.client.http.HttpBackOffIOExceptionHandler;
import com.google.api.client.http.HttpBackOffUnsuccessfulResponseHandler;
import com.google.api.client.http.HttpBackOffUnsuccessfulResponseHandler.BackOffRequired;
import com.google.api.client.http.HttpHeaders;
import com.google.api.client.http.HttpRequest;
import com.google.api.client.http.HttpRequestFactory;
import com.google.api.client.http.HttpResponse;
import com.google.api.client.http.HttpStatusCodes;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.UrlEncodedContent;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.JsonFactory;
import com.google.api.client.json.JsonObjectParser;
import com.google.api.client.json.jackson2.JacksonFactory;
import com.google.api.client.json.webtoken.JsonWebSignature;
import com.google.api.client.json.webtoken.JsonWebToken;
import com.google.api.client.util.ExponentialBackOff;
import com.google.api.client.util.GenericData;
import com.google.auth.oauth2.ServiceAccountCredentials;
import com.google.common.base.MoreObjects;
import com.google.common.io.BaseEncoding;

public class GoogleIDToken {

     private static final String OAUTH_TOKEN_URI = "https://www.googleapis.com/oauth2/v4/token";
     private static final String GOOGLE_CERT_URL = "https://www.googleapis.com/oauth2/v3/certs";
     private static final String METADATA_IDENTITY_DOC_URL = "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity";
     private static final String GRANT_TYPE = "urn:ietf:params:oauth:grant-type:jwt-bearer";
     private static final String PARSE_ERROR_PREFIX = "Error parsing token refresh response. ";

     public static void main(String[] args) throws Exception {

          String target_audience = "https://foo.com";

          GoogleIDToken tc = new GoogleIDToken();

          String credFile = "/path/to/svc.json";
          ServiceAccountCredentials sac = ServiceAccountCredentials.fromStream(new FileInputStream(credFile));

          IdToken tok = tc.getIDTokenFromServiceAccount(sac, target_audience);
          System.out.println(tok);
          System.out.println(GoogleIDToken.verifyToken(tok.getTokenValue(), target_audience));

          // IdToken tok = tc.getIDTokenFromComputeEngine(target_audience);
          // System.out.println(tok);
          // System.out.println(GoogleIDToken.verifyToken(tok.getTokenValue(),
          // target_audience));

          String url = "https://foo-endpont.a.run";
          tc.MakeAuthenticatedRequest(tok.getTokenValue(), url);
     }

     public IdToken getIDTokenFromServiceAccount(ServiceAccountCredentials sc, String targetAudience) throws Exception {
          try {

               HttpTransport httpTransport = new NetHttpTransport();
               JacksonFactory jsonFactory = new JacksonFactory();
               long currentTime = new Date().getTime();
               String assertion = createAssertionForIdToken(sc, jsonFactory, currentTime, OAUTH_TOKEN_URI,
                         targetAudience);

               GenericData tokenRequest = new GenericData();
               tokenRequest.set("grant_type", GRANT_TYPE);
               tokenRequest.set("assertion", assertion);
               UrlEncodedContent content = new UrlEncodedContent(tokenRequest);

               HttpRequestFactory requestFactory = httpTransport.createRequestFactory();
               HttpRequest request = requestFactory.buildPostRequest(new GenericUrl(OAUTH_TOKEN_URI), content);
               request.setParser(new JsonObjectParser(jsonFactory));

               HttpResponse response;
               try {
                    response = request.execute();
               } catch (IOException e) {
                    throw new IOException(
                              String.format("Error getting idToken for service account: %s", e.getMessage()), e);
               }

               GenericData responseData = response.parseAs(GenericData.class);
               String rawToken = validateString(responseData, "id_token", PARSE_ERROR_PREFIX);

               if (rawToken.split("\\.").length !=3) 
                 throw new Exception("Unable to parse segments of IDToken");
               String payload = rawToken.split("\\.")[1];
               String decodedToken = new String(BaseEncoding.base64().decode(payload), StandardCharsets.UTF_8);
               JsonWebToken.Payload jwtPayload = jsonFactory.fromString(decodedToken, JsonWebToken.Payload.class);
               return new IdToken(rawToken, new Date(jwtPayload.getExpirationTimeSeconds() * 1000),
                         jwtPayload.getAudienceAsList());
          } catch (IOException ex) {
               throw new IOException("Unable to Parse IDToken " + ex.getMessage(), ex);
          }
     }

     private static String validateString(Map<String, Object> map, String key, String errorPrefix) throws IOException {
          Object value = map.get(key);
          if (value == null) {
               throw new IOException(String.format("VALUE not found during prsing", errorPrefix, key));
          }
          if (!(value instanceof String)) {
               throw new IOException(String.format("Wrong type parsed: ", errorPrefix, "string", key));
          }
          return (String) value;
     }

     private String createAssertionForIdToken(ServiceAccountCredentials sc, JsonFactory jsonFactory, long currentTime,
               String audience, String targetAudience) throws IOException {
          JsonWebSignature.Header header = new JsonWebSignature.Header();
          header.setAlgorithm("RS256");
          header.setType("JWT");
          header.setKeyId(sc.getPrivateKeyId());

          JsonWebToken.Payload payload = new JsonWebToken.Payload();
          payload.setIssuer(sc.getClientEmail());
          payload.setIssuedAtTimeSeconds(currentTime / 1000);
          payload.setExpirationTimeSeconds(currentTime / 1000 + 3600);
          payload.setSubject(sc.getServiceAccountUser());

          if (audience == null) {
               payload.setAudience(OAUTH_TOKEN_URI.toString());
          } else {
               payload.setAudience(audience);
          }

          payload.set("target_audience", targetAudience);

          String assertion;
          try {
               assertion = JsonWebSignature.signUsingRsaSha256(sc.getPrivateKey(), jsonFactory, header, payload);
          } catch (GeneralSecurityException e) {
               throw new IOException("Error signing service account access token request with private key.", e);
          }
          return assertion;
     }

     public IdToken getIDTokenFromComputeEngine(String audience) throws Exception {

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

          String rawToken = response.parseAsString();
          response.disconnect();

          if (rawToken.split("\\.").length !=3) 
            throw new Exception("Unable to parse segments of IDToken");
          String payload = rawToken.split("\\.")[1];
          String decodedToken = new String(BaseEncoding.base64().decode(payload), StandardCharsets.UTF_8);
          JacksonFactory jsonFactory = new JacksonFactory();
          JsonWebToken.Payload jwtPayload = jsonFactory.fromString(decodedToken, JsonWebToken.Payload.class);
          return new IdToken(rawToken, new Date(jwtPayload.getExpirationTimeSeconds() * 1000),
                    jwtPayload.getAudienceAsList());
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
          int statusCode = response.getStatusCode();
          if (statusCode != HttpStatusCodes.STATUS_CODE_OK) {
               throw new IOException(String.format("Unable to get IDToken from metadataServer %s: %s", statusCode,
                         response.parseAsString()));
          }

          System.out.println(response.parseAsString());
          response.disconnect();

     }

}

class IdToken {

     private final String tokenValue;
     private final List<String> audience;
     private final Long expirationTimeMillis;

     public IdToken(String tokenValue, Date expirationTime, List<String> audience) {
          this.tokenValue = tokenValue;
          this.expirationTimeMillis = (expirationTime == null) ? null : expirationTime.getTime();
          this.audience = audience;
     }

     public String getTokenValue() {
          return tokenValue;
     }

     public List<String> getAudience() {
          return audience;
     }

     public Date getExpirationTime() {
          if (expirationTimeMillis == null) {
               return null;
          }
          return new Date(expirationTimeMillis);
     }

     Long getExpirationTimeMillis() {
          return expirationTimeMillis;
     }

     public String toString() {
          return MoreObjects.toStringHelper(this).add("tokenValue", tokenValue)
                    .add("expirationTimeMillis", expirationTimeMillis).toString();
     }
}
