import os
import google
from google.appengine.ext import vendor

lib_directory = os.path.dirname(__file__) + '/lib'


google.__path__ = [os.path.join(lib_directory, 'google')] + google.__path__

vendor.add(lib_directory)
