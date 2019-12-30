package com.test;
import java.util.ArrayList;
import java.util.List;
import java.util.logging.ConsoleHandler;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.logging.SimpleFormatter;

import com.google.api.client.googleapis.auth.oauth2.GoogleCredential;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.jackson2.JacksonFactory;

import org.apache.commons.codec.binary.Base64;

import java.io.FileInputStream;
import java.io.IOException;
import java.net.URLEncoder;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.Signature;

import com.google.api.services.pubsub.Pubsub;
import com.google.api.services.pubsub.model.ListTopicsResponse;
import com.google.api.services.pubsub.model.PublishRequest;
import com.google.api.services.pubsub.model.PublishResponse;
import com.google.api.services.pubsub.model.PubsubMessage;
import com.google.api.services.pubsub.model.Topic;
import com.google.api.services.pubsub.Pubsub;
import com.google.api.services.pubsub.model.PublishRequest;
import com.google.api.services.pubsub.model.PublishResponse;
import com.google.api.services.pubsub.model.PubsubMessage;
import com.google.common.collect.ImmutableList;
import com.google.api.services.pubsub.PubsubScopes;

	public class CloudPubSubClient {

		
		public CloudPubSubClient() {
			try {
				
		        ConsoleHandler consoleHandler = new ConsoleHandler();
		        consoleHandler.setLevel(Level.ALL);
		        consoleHandler.setFormatter(new SimpleFormatter());
		        
		        Logger logger = Logger.getLogger("com.google.api.client");
		        logger.setLevel(Level.ALL);
		        logger.addHandler(consoleHandler);  
		        
				Logger lh = Logger.getLogger("httpclient.wire.header");
				lh.setLevel(Level.ALL);
				lh.addHandler(consoleHandler);
				
				Logger lc = Logger.getLogger("httpclient.wire.content");
				lc.setLevel(Level.ALL);
				lc.addHandler(consoleHandler);		
				
			  HttpTransport httpTransport = new NetHttpTransport();
			  JacksonFactory jsonFactory = new JacksonFactory();

			GoogleCredential credential = GoogleCredential.getApplicationDefault();	
				   
				  credential.refreshToken();
				  String accessToken = credential.getAccessToken();
				  System.out.println(accessToken);
				  credential.setAccessToken("foo");
				   Pubsub pubsub = new Pubsub.Builder(httpTransport, jsonFactory, credential).build();

				  Pubsub.Projects.Topics.Create cm = pubsub.projects().topics().create("projects/p0/topics/t0", new Topic());
				  cm.execute();
				  
				  Pubsub.Projects.Topics.List listMethod =
					        pubsub.projects().topics().list("projects/p0");
					String nextPageToken = null;
					ListTopicsResponse response;
					do {
					    if (nextPageToken != null) {
					        listMethod.setPageToken(nextPageToken);
					    }
					    response = listMethod.execute();
					    List<Topic> topics = response.getTopics();
					    if (topics != null) {
					        for (Topic topic : topics) {
					            System.out.println("Found topic: " + topic.getName());
					        }
					    }
					    nextPageToken = response.getNextPageToken();
					} while (nextPageToken != null);
				   
				   String message = "Hello Cloud Pub/Sub!";
				   PubsubMessage pubsubMessage = new PubsubMessage();

				   pubsubMessage.encodeData(message.getBytes("UTF-8"));
				   List<PubsubMessage> messages = ImmutableList.of(pubsubMessage);
				   PublishRequest publishRequest =  new PublishRequest().setMessages(messages);
				   PublishResponse publishResponse = pubsub.projects().topics()
				           .publish("projects/p0/topics/t0", publishRequest)
				           .execute();
				   List<String> messageIds = publishResponse.getMessageIds();
				   if (messageIds != null) {
				       for (String messageId : messageIds) {
				           System.out.println("messageId: " + messageId);
				       }
				   }
				   
				   
			}
			catch (Exception ex) {
				System.out.println("Error : " + ex);
			}
		}
		

	    
	}
