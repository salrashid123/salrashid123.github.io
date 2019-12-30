from google.oauth2 import id_token
from google.oauth2 import service_account
import google.auth
import google.auth.transport.requests
from google.auth.transport.requests import AuthorizedSession

target_audience = 'https://myapp-6w42z6vi3q-uc.a.run.app'
url = 'https://myapp-6w42z6vi3q-uc.a.run.app'
certs_url='https://www.googleapis.com/oauth2/v1/certs'

additional_claims = { }

creds = service_account.IDTokenCredentials.from_service_account_file(
        '/path/to/cert.json',
        target_audience= target_audience, additional_claims=additional_claims)

authed_session = AuthorizedSession(creds)

# make authenticated request

r = authed_session.get(url)

print r.status_code
print r.text


# verify

request = google.auth.transport.requests.Request()
idt = creds.token
print idt
print id_token.verify_token(idt,request,certs_url=certs_url)