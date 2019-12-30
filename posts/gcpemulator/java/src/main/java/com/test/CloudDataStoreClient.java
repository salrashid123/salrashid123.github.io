package com.test;
import java.util.ArrayList;
import java.util.logging.ConsoleHandler;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.logging.SimpleFormatter;

import com.google.api.client.googleapis.auth.oauth2.GoogleCredential;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.jackson2.JacksonFactory;

import com.google.datastore.v1beta3.BeginTransactionRequest;
import com.google.datastore.v1beta3.BeginTransactionResponse;
import com.google.datastore.v1beta3.CommitRequest;
import com.google.datastore.v1beta3.CommitResponse;
import com.google.datastore.v1beta3.Entity;
import com.google.datastore.v1beta3.Key;
import com.google.datastore.v1beta3.LookupRequest;
import com.google.datastore.v1beta3.LookupResponse;
import com.google.datastore.v1beta3.Value;
import com.google.datastore.v1beta3.client.Datastore;
import com.google.datastore.v1beta3.client.DatastoreException;
import com.google.datastore.v1beta3.client.DatastoreFactory;
import com.google.datastore.v1beta3.client.DatastoreHelper;
import com.google.datastore.v1beta3.client.DatastoreOptions;
import com.google.protobuf.ByteString;


public class CloudDataStoreClient {



	
	public CloudDataStoreClient() {
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
			  	credential.setAccessToken("foo");
			    String myprojectId = "p0";

                DatastoreOptions options = new DatastoreOptions.Builder().projectId(myprojectId).credential(credential).build();
                Datastore datastore =   DatastoreFactory.get().create(options);

			      LookupRequest.Builder lreq = LookupRequest.newBuilder();

			      Key.Builder key = Key.newBuilder().addPath(
			          Key.PathElement.newBuilder().setKind("Employee").setName("aguadypoogznoqofmgmy"));

			      lreq.addKeys(key);
			      LookupResponse lresp = datastore.lookup(lreq.build());
			      if (lresp.getFoundCount() > 0) {
			        Entity e = lresp.getFound(0).getEntity();
			        System.out.println(e);
			      }
			      else {
			    	  
			          BeginTransactionRequest.Builder treq = BeginTransactionRequest.newBuilder();
			          BeginTransactionResponse tres = datastore.beginTransaction(treq.build());
			          ByteString tx = tres.getTransaction();			    	  
			          CommitRequest.Builder creq = CommitRequest.newBuilder();			    	  
			    	  creq.setTransaction(tx);
			    	  
			    	  Entity entity;
			          Entity.Builder entityBuilder = Entity.newBuilder();
			          entityBuilder.setKey(key);

			          entityBuilder.getMutableProperties().put("some_string", Value.newBuilder().setStringValue("some value").build());

			          entity = entityBuilder.build();
			          creq.addMutationsBuilder().setInsert(entity);		
			          datastore.commit(creq.build());
			      }

		}
		catch (Exception ex) {
			System.out.println("Error : " + ex);
		}
	}

}
