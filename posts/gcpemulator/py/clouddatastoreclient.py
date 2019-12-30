#!/usr/bin/python


from apiclient.errors import HttpError
from oauth2client.client import AccessTokenRefreshError
import googledatastore as datastore
from googledatastore.helper import *
from oauth2client.client import GoogleCredentials
import json
import pprint
import sys
import httplib2
import logging


def clouddatastore_client():

  try:

    logFormatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')                              
    root = logging.getLogger()
    root.setLevel(logging.INFO)           
    ch = logging.StreamHandler(sys.stdout)
    ch.setLevel(logging.INFO)    
    ch.setFormatter(logFormatter)
    root.addHandler(ch)
    logging.getLogger('oauth2service.client').setLevel(logging.INFO)
    logging.getLogger('apiclient.discovery').setLevel(logging.INFO)
             
    credentials = GoogleCredentials.get_application_default()
      
    http = httplib2.Http()
    http = credentials.authorize(http)
    
    credentials.refresh(http)
    credentials.access_token = 'foo'
    print credentials.access_token

    datastore.set_options(project_id='p0', credentials=credentials)
    

    req = datastore.LookupRequest()
    key = datastore.Key()
    path = key.path.add()
    path.kind = 'Employee'
    path.name = 'aguadypoogznoqofmgmy'
    req.keys.extend([key])
    
    resp = datastore.lookup(req)
    if resp.found:
      entity = resp.found[0].entity 
      print (str(entity))  
      for prop in entity.properties:
        print 'Lookup: ' + str(prop)
    else:
      print 'entity not found; initialize entity and insert..'
      req = datastore.CommitRequest()
      req.mode = datastore.CommitRequest.NON_TRANSACTIONAL
      employee = req.mutations.add().insert
      path = employee.key.path.add()
      path.kind = 'Employee'
      path.name = 'aguadypoogznoqofmgmy'
      res = datastore.commit(req)             
      print res
      
  except HttpError as err:
    print 'Error:', pprint.pprint(err.content)

  except AccessTokenRefreshError:
    print ("Credentials have been revoked or expired, please re-run"
           "the application to re-authorize")

if __name__ == '__main__':
  clouddatastore_client()
