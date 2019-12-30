FROM ubuntu

RUN apt-get -y update
RUN apt-get install -y curl python openjdk-8-jdk supervisor build-essential ca-certificates nginx
RUN mkdir -p /var/log/supervisor
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

RUN curl https://sdk.cloud.google.com | bash

COPY nginx.conf /etc/nginx/nginx.conf
COPY certs /etc/nginx/certs
COPY html /usr/share/nginx/html
RUN  chown -R nobody:root /usr/share/nginx/html
 
ENV CLOUDSDK_CORE_DISABLE_PROMPTS 1

ENV CLOUDSDK_CORE_PROJECT p0

RUN /root/google-cloud-sdk/bin/gcloud config set project $CLOUDSDK_CORE_PROJECT
RUN /root/google-cloud-sdk/bin/gcloud components install beta
RUN /root/google-cloud-sdk/bin/gcloud components install cloud-datastore-emulator gcd-emulator pubsub-emulator

EXPOSE 443 8283 8490 

CMD ["/usr/bin/supervisord"]
