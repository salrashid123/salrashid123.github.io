


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