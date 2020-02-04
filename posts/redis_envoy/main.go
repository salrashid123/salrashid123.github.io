package main

import (
	"fmt"

	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/go-redis/redis/v7"
)

var ()

func main() {

	caCert, err := ioutil.ReadFile("CA_crt.pem")
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cer, err := tls.LoadX509KeyPair("client.crt", "client.key")
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	config := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cer},
		ServerName:   "server.domain.com",
	}

	client := redis.NewClient(&redis.Options{
		Addr:      "server.domain.com:6000", // "server.domain.com:6379",
		Password:  "bar",
		DB:        0, // use default DB
		TLSConfig: config,
	})

	pong, err := client.Ping().Result()
	if err != nil {
		fmt.Printf("Unable to ping %v\n", err)
		return
	}
	fmt.Println(pong, err)

	err = client.Set("key", "value", 0).Err()
	if err != nil {
		fmt.Printf("Unable to ping %v\n", err)
		return
	}

	val, err := client.Get("key").Result()
	if err != nil {
		fmt.Printf("Unable to ping %v\n", err)
		return
	}
	fmt.Println("key", val)

}
