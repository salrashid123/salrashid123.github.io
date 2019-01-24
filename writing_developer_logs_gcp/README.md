# Writing Developer logs with Google Cloud Logging

Several months ago Google Cloud Logging introduced two new [monitored resource](https://cloud.google.com/logging/docs/basic-concepts#monitored-resources) types geared towards allowing developers to emit cloud logging messages specifically for their own application centric logs.  Pereviously, application logs generally had to be tied to existing predefined `monitored_resources` such as `GCE`, `GKE`, `AppEngine`, `Dataflow` and so on.  Under those monitoried resources sources, multiple log entries were attributed to to specific `logNames` describing the subsytem like `syslog`, `apache2`, `nginx`, `mysql`, etc.  In the end, the `monitored_resource` defined the source system and the `logName` the source application/system.

What if you wanted to emit your own application logs that allows you to define a generic source resource _and_ the logName?  In other words, have a monitoired resource that specifically doens't prescribe the source system but allows you to define describe your own.  Thats where the new `generic_node` and `generic_task` resource types come in. These new resource types can be thought of as a generic source system that represents where your application runs (`generic_node`) and the specific instance of your application (`generic_task`).

This article will describe the two new types and how to send log to GCP using both the GCP Logging API, the GCP logging agent and finally the off the shelf `fluentd` agent alone.

So...what are these two new types?

## Generic Node

Taken from the documentation:

- [Generic Node](https://cloud.google.com/monitoring/api/resources#tag_generic_node):

> "A generic node identifies a machine or other computational resource for which no more specific resource type is applicable. The label values must uniquely identify the node."

under which you can define some lables:

* `project_id`: The identifier of the GCP project associated with this resource, such as "my-project".
* `location`: The GCP or AWS region in which data about the resource is stored. For example, "us-east1-a" (GCP) or "aws:us-east-1a" (AWS).
* `namespace`: A namespace identifier, such as a cluster name.
* `node_id`: A unique identifier for the node within the namespace, such as a hostname or IP address. 

## Generic Task

Taken from the documentation:

- [Generic Task](https://cloud.google.com/monitoring/api/resources#tag_generic_task):

> "A generic task identifies an application process for which no more specific resource is applicable, such as a process scheduled by a custom orchestration system. The label values must uniquely identify the task."

* `project_id`: The identifier of the GCP project associated with this resource, such as "my-project".
* `location`: The GCP or AWS region in which data about the resource is stored. For example, "us-east1-a" (GCP) or "aws:us-east-1a" (AWS).
* `namespace`: A namespace identifier, such as a cluster name.
* `job`: An identifier for a grouping of related tasks, such as the name of a microservice or distributed batch job.
* `task_id`: A unique identifier for the task within the namespace and job, such as a replica index identifying the task within the job. 


## Sending Logs

This article will show how to send `generic_task` and `generic_node` logs via

* Cloud Logging API
  - Emit logs via the API from anywhere.
  - Requires [Authentication Default Credentials](https://cloud.google.com/docs/authentication/production).

* Cloud Logging Agent
  - Supported on GCE/EC2 and uses `google-fluentd`,
  - Can acquire authentication credentials from `metadata sever` on GCE.
  - Requires authentication credentials file on EC2.

* Generic fluentd
  - Supported anywhere `fluentd` runs.
  - Requires authentication credentials file.

If you are running on GCE or EC2, the `Cloud Logging Agent` would be the best bet.  If you are already using `fluentd` elsewhere, adapt the generic agent and add the `gem` file.

For refernece, here are some related articles on Cloud Logging I tried out over the year:
  - [Apache/Nginx Structured Logs with Cloud Logging Agent](https://medium.com/google-cloud/envoy-nginx-apache-http-structured-logs-with-google-cloud-logging-91fe5965badf)
  - [Flask Logging plugin for Combined logs](https://github.com/salrashid123/flask-gcp-log-groups)
  - [Combining correlated Log Lines in Google Stackdriver](https://medium.com/google-cloud/combining-correlated-log-lines-in-google-stackdriver-dd23284aeb29)
  - [auditd agent config for Stackdriver Logging](https://medium.com/google-cloud/auditd-agent-config-for-stackdriver-logging-c27d1431ed3a)
  - [Envoy Proxy fluentd parser](https://github.com/salrashid123/fluent-plugin-envoy-parser)

### Cloud Logging API

The cloud logging API is pretty simple to use.  First setup `Appplication Default Credentials` by running `gcloud auth application-default login`.  Then alter the `project_id` setting below and execute the script

```python
from google.cloud import logging
from google.cloud.logging.resource import Resource

client = logging.Client()

logger = client.logger('nodelog_1')
r = Resource("generic_node", labels={
    'project_id':'fabled-ray-104117',
    'location': 'us-central1-a',
    'namespace': 'default',
    'node_id': '10.10.10.1'})
logger.log_struct(
    {"message": "My first entry", "weather": "partly cloudy"}, resource=r
)


logger = client.logger('tasklog_1')
r = Resource("generic_task",  labels={
    'location': 'us-central1-a',
    'namespace': 'default',
    'job':'some job',
    'task_id': '12345'} )
logger.log_struct(
    {"message2": "My second entry", "weather": "still partly cloudy"}, resource=r
)
```
Gives:

- ![images/generic_node_api.png](images/generic_node_api.png)

and 

- ![images/generic_task_api.png](images/generic_task_api.png)

## Cloud Logging Agent

Apart from the API, Cloud Logging can consume logs emitted by a generic fluentd agent when configured to use [flutnet-plugin-google-cloud](https://github.com/GoogleCloudPlatform/fluent-plugin-google-cloud) or while on GCE/EC2, via the google [cloud logging agent](https://cloud.google.com/logging/docs/agent/).  The cloud logging agent is actually still just `fluentd` but with GCP-centric parsers and built in and an installer.

First we will install the `cloud logging agent` on GCE and then the `fluent-plugin-google-cloud` on generic, stan-alone `fluentd`.  This article does not show the installation steps on AWS and the links above describe its setup.

### GCE

1) Create VM with `Logging Writer` scope enabled

2) `ssh` to the VM and install the structured logging agent

```
    curl -sSO "https://dl.google.com/cloudagents/install-logging-agent.sh"
    sudo bash install-logging-agent.sh --structured
```

3) Add configuration to `/etc/google-fluentd/google-fluentd.conf`

> Note: the `@type http` was added in just for testing; in real usecases, you will setup the log file source as described at the end of this article.

```
  <source>
    @type http
    @id input_http
    port 8888
  </source>

  <match gcp_resource.**>
    @type google_cloud
    monitored_resource generic_node
    namespace default
  </match>

  <match gcp_task.**>
    @type google_cloud
    monitored_resource generic_task
    namespace default
    job random_job
    task_id random_task
  </match>
```

4) Restart `google-fluentd`

```
 service google-fluentd restart
```

5) Send sample traffic to http test source:

```
curl -X POST -d 'json={"foo":"bar"}' http://localhost:8888/gcp_resource.log
curl -X POST -d 'json={"foo":"bar"}' http://localhost:8888/gcp_task.log
```

6) Check cloud logging console for output under `Generic Node` and `Generic Task`

- ![images/generic_node_agent.png](images/generic_node_agent.png)

- ![images/generic_task_agent.png](images/generic_task_agent.png)


Note that in the configuration above we did not define the `node_id` or `location`.  If those settings are not specified the google fluentd agent will automatically try to derive the values from GCE or EC2 metadata server.  In the example above, the GCE vm_id is uses as the `node_id`.


### Fluentd

The following describes installing [fluent-plugin-google-cloud](https://github.com/GoogleCloudPlatform/fluent-plugin-google-cloud) into generic `fluentd`:

1) Create a `service_account` and JSON key on a GCP project.
2) Assign IAM role _Logging Writer_ to that service account
3) Download JSON cert and rename the file to `application_default_credentials.json`

4) Install `fluentd` on the target system (in this case `debian-stretch`)

```
  curl -L https://toolbelt.treasuredata.com/sh/install-debian-stretch-td-agent3.sh | sh
```

5) Install `fluent-plugin-google-cloud` gem

```
  /opt/td-agent/embedded/bin/gem install fluent-plugin-google-cloud
```

6) Add certificate and change permisssions

  Copy  `application_default_credentials.json` to the target system as `/etc/google/auth/application_default_credentials.json` and change permissions:

```bash
  $ chown td-agent:td-agent /etc/google/auth/application_default_credentials.json
  $ chmod go-rwx /etc/google/auth/application_default_credentials.json

  $ ls -lart /etc/google/auth/application_default_credentials.json
  -rw-r----- 1 td-agent td-agent 2332 Jan 16 20:52 /etc/google/auth/application_default_credentials.json
```

7) Configure fluentd
  Edit `/etc/td-agent/td-agent.conf` and configurations for `generic_node` and `generic_task`:

```
    <source>
      @type http
      @id input_http
      port 8888
    </source>

    <match gcp_resource.**>
      @type google_cloud
      use_metadata_service false
      monitored_resource generic_node
      location us-central1-a
      namespace default
      node_id 10.10.10.1
    </match>

    <match gcp_task.**>
      @type google_cloud
      use_metadata_service false
      monitored_resource generic_task
      location us-central1-a
      namespace default
      job random_job
      task_id random_task
    </match>
```

> again, we're using  a test `@type http` source just as a demo.

8) Restart fluentd

```
  service td-agent restart
```

- Send sample traffic to http listener

```
  curl -X POST -d 'json={"foo":"bar"}' http://localhost:8888/gcp_resource.log
  curl -X POST -d 'json={"foo":"bar"}' http://localhost:8888/gcp_task.log
```

9) Check cloud logging console for output under `Generic Node` and `Generic Task`

- ![images/generic_node_agent_fluent.png](images/generic_node_agent_fluent.png)

- ![images/generic_task_agent_fluent.png](images/generic_task_agent_fluent.png)


> **Note** If you do not not define the `node_id` or `location` and run on GCE or EC2, the plugin will attempt to automatically try to derive the values from GCE or EC2 metadata server for the`vm_id` as the `node_id` and `zone` as `location`.


## Developer logs from logfiles

The example above we setup the cloud logging agent for GCE and the plugin for fluentd but used a test debug handler to source logs (`@type http`).  In the following configuration, we'll use an actual log file (eg. `/var/log/node_logs.log`) (which is what you'd likely do in production).  The following isn't anything specific to `google-fluentd` but rather just plain fluentd configuration for log sources (i've added in the section in to show a more realistic example than `@type http`).

### GCE

In this example all we are doing is setting up a log file to track and taged as `gcp_resource` which in our config is handled by `google_cloud` as a `generic_node`:

- `/etc/google-fluentd/google-fluentd.conf`

```
  <match gcp_resource.**>
    @type google_cloud
    monitored_resource generic_node
    namespace default
  </match>
```

- `/etc/google-fluentd/config.d/mynode.conf`

```
  <source>
    @type tail
    format /(?<action>\w+) (?<thing>\w+)/
    tag gcp_resource
    read_from_head true
    path /var/log/node_logs.log
    pos_file /var/lib/google-fluentd/pos/node_logs.pos
  </source>
```

> note, i've seutp a regex parse the logs as JSON as shown in the link below

then

```
echo "hello world" > /var/log/node_logs.log
```

gives

- ![images/generic_node_agent_gcp_logfile.png](images/generic_node_agent_gcp_logfile.png)

For more info see [Writing your own parser](https://cloud.google.com/logging/docs/structured-logging#writing_your_own_parser) and [Customizing Agent Configuration](https://cloud.google.com/logging/docs/agent/configuration#configure)

### Fluentd

Edit  `/etc/td-agent/td-agent.conf` and add:

```
  <match gcp_resource.**>
    @type google_cloud
    use_metadata_service false
    monitored_resource generic_node
    location us-central1-a
    namespace default
    node_id 10.10.10.1
  </match>

  <source>
    @type tail
    format /(?<action>\w+) (?<thing>\w+)/
    tag gcp_resource
    read_from_head true
    path /var/log/node_logs.log
    pos_file /var/lib/td-agent/node_logs.pos
  </source>
```

(You my need to setup permissions for fluentd on the pos and log file: 
`chown td-agent:td-agent /var/lib/td-agent/node_logs.pos`)

then after restart of `td-agent`

```
echo "hello world" > /var/log/node_logs.log
```

You should see the logs in GCP assuming you setup a JSON certficate file into (`/etc/google/auth/application_default_credentials.json`)
- ![images/generic_node_agent_fluent_logfile.png](images/generic_node_agent_fluent_logfile.png)


## Summary

You can use these new types to setup logs for your own application and ingest them into `Google Cloud Logging`.  While you don't get truly 'top-level' resource types (your'e still dealing with the `generic_*` ones), you are free here to define what system and application that sources logs.  Once you have logs with these fields sent, you can setup additional GCP capabilities like [logs-to-metrics](https://cloud.google.com/logging/docs/logs-based-metrics/), alerts, and so on based on your own applicaiton logs and devOPS needs.
