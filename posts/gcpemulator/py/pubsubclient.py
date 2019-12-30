#!/usr/bin/python
import pprint
from apiclient.discovery import build
import httplib2
from oauth2client.file import Storage
import logging
import sys
import os
import json
from oauth2client.client import GoogleCredentials

logFormatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')                              
root = logging.getLogger()
root.setLevel(logging.INFO)           
ch = logging.StreamHandler(sys.stdout)
ch.setLevel(logging.INFO)    
ch.setFormatter(logFormatter)
root.addHandler(ch)
logging.getLogger('oauth2service.client').setLevel(logging.INFO)
logging.getLogger('apiclient.discovery').setLevel(logging.INFO)   

scope = 'https://www.googleapis.com/auth/pubsub'

#storage = Storage('creds.dat')
credentials = None
#credentials = storage.get()

if credentials is None or credentials.invalid:

  credentials = GoogleCredentials.get_application_default()
  credentials = credentials.create_scoped(scope)
  
  #storage.put(credentials)


DISCOVERY_URI = "https://www.googleapis.com/discovery/v1/apis/pubsub/v1/rest"

http = httplib2.Http()
#http = httplib2.Http(disable_ssl_certificate_validation=True)

http = credentials.authorize(http)
credentials.access_token = 'foo'

httplib2.debuglevel=0
service = build('pubsub', version='v1', http=http)
topic = service.projects().topics().create(name='projects/p0/topics/t0', body={}).execute()
#service = build('pubsub', 'v1',discoveryServiceUrl=DISCOVERY_URI, http=http)
request = service.projects().topics().list(project='projects/p0')
response = request.execute(http=http)
print json.dumps(response, sort_keys=True, indent=4)



