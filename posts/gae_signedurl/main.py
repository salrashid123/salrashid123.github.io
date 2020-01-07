
from google.appengine.ext import webapp
import json 
from datetime import datetime
import time
from datetime import timedelta
import urllib
import base64

from google.appengine.api import app_identity



class MainPage(webapp.RequestHandler):
  def get(self):

    BUCKET_NAME="mineral-minutia-820"
    object_name="foo.txt"
    expiration = int(time.mktime((datetime.now() +
                                  timedelta(days=1)).timetuple()))

    signature_string = 'GET\n\n\n{}\n{}'.format(str(expiration),
                                                '/' + BUCKET_NAME + '/' +
                                                urllib.quote(object_name))
    signature_signed = self.sign(signature_string)

    urlt = 'https://storage.googleapis.com/{}/{}?{}'
    service_account = app_identity.get_service_account_name()
    url = urlt.format(urllib.quote(BUCKET_NAME),
                      urllib.quote(object_name),
                      urllib.urlencode({'GoogleAccessId': service_account,
                                        'Expires': str(expiration),
                                        'Signature': signature_signed}))
    self.response.write(json.dumps({'url': url}))

  def sign(self, plaintext):
      signature_bytes = app_identity.sign_blob(plaintext)[1]
      return base64.b64encode(signature_bytes)

class HealthCheck(webapp.RequestHandler):
    def get(self):
        self.response.set_status(200)
        self.response.out.write('ok')

app = webapp.WSGIApplication([('/', MainPage), ('/_ah/health',HealthCheck)], debug=True)


