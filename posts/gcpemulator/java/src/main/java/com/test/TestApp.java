package com.test;

import java.util.Arrays;
import java.util.Collections;
import java.io.*;
import com.google.api.client.googleapis.auth.oauth2.GoogleCredential;
import com.google.api.client.auth.oauth2.Credential;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.jackson2.JacksonFactory;
import com.google.api.services.oauth2.Oauth2;
import com.google.api.services.oauth2.Oauth2Request;
import com.google.api.services.oauth2.Oauth2RequestInitializer;
import com.google.api.services.oauth2.Oauth2Scopes;
import com.google.api.services.oauth2.model.Userinfoplus;

public class TestApp {
	public static void main(String[] args) {
		TestApp tc = new TestApp();
	}
		
	public TestApp() {
		try
		{

			System.out.println("Starting..");
			CloudPubSubClient tc = new CloudPubSubClient();
		    //CloudDataStoreClient tc = new CloudDataStoreClient();

		} 
		catch (Exception ex) {
			System.out.println("Error:  " + ex);
		}
	}
	    
}
