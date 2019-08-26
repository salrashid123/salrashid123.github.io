#
# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import google.oauth2.credentials
from google.oauth2 import id_token
from google.auth import impersonated_credentials
from google.oauth2 import service_account
import google.auth
import google.auth.transport.requests
from google.auth.transport.requests import AuthorizedSession

# pip install google-auth requests

target_audience = 'https://example.com'

url = 'https://cloud-run-url.example.com'
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

def GetIDTokenFromImpersonatedCredentials(svcAccountFile, target_audience):
  source_credentials = service_account.Credentials.from_service_account_file(svcAccountFile)
  target_scopes = ['https://www.googleapis.com/auth/cloud-platform']  

  target_credentials = impersonated_credentials.Credentials(
      source_credentials = source_credentials,
      target_principal='impersonated-account@fabled-ray-104117.iam.gserviceaccount.com',
      target_scopes = target_scopes,
      delegates=[],
      lifetime=300)

  id_creds = impersonated_credentials.IDTokenCredentials(
      target_credentials, target_audience=target_audience, include_email=False)

  request = google.auth.transport.requests.Request()
  id_creds.refresh(request)
  return id_creds.token

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

# For Impersonated Credentials
#token = GetIDTokenFromImpersonatedCredentials(svcAccountFile, target_audience)


print 'Token: ' + token
if VerifyIDToken(token=token,certs_url=certs_url, audience=target_audience):
  print 'token Verified with aud: ' + target_audience
print 'Making Authenticated API call:'
MakeAuthenticatedRequest(token,url)
