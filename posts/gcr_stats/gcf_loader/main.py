import os
import traceback

from google.api_core import retry
from google.cloud import bigquery

PROJECT_ID = os.getenv('GCP_PROJECT')
dataset_id = 'storageanalysis'
table_id = 'usage'

bigquery_client = bigquery.Client()

def loader(data, context):
    bucket_name = data['bucket']
    file_name = data['name']

    # check if we've got an access log or storage log (access logs have 'usage' in the name)
    if 'usage' in file_name:
        load_job = None
        try:
            dataset_ref = bigquery_client.dataset(dataset_id)
            job_config = bigquery.LoadJobConfig()
            job_config.skip_leading_rows = 1
            job_config.source_format = bigquery.SourceFormat.CSV
            uri = "gs://{}/{}".format(bucket_name,file_name) 
            load_job = bigquery_client.load_table_from_uri(
                uri, dataset_ref.table(table_id), job_config=job_config
            )
            print("Starting job {}".format(load_job.job_id))

            load_job.result()
            print("Job finished.")

            destination_table = bigquery_client.get_table(dataset_ref.table(table_id))
            print("Loaded {} rows.".format(destination_table.num_rows))
        except Exception as e:
            print(e)
            print(load_job.errors)
    return 