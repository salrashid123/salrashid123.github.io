# Private Access to GCP APIs through VPN Tunnels

This tutorial demonstrates how to use APIs for Google Cloud Platform (GCP) services from an external network, such as your on-premises private network or another cloud provider's network. This approach allows your on-premises servers that are connected to your private network to access GCP services without using public IP addresses.

GCP customers often have workloads spanning cloud proividers and their on-premises datacenters connected via VPN.  In many of those 
situations, customers need to access various Google Cloud APIs where arbitrary outbound traffic from
any system is restricted or acutely controlled.  

This not a problem while the workload is running on GCP:  accessing GCP APIs from within a VPC is directed
towards Google-internal interfaces where those services resides.  When accessing these same APIs from other restricted cloud providers or 
even your own datacenter the situation is a bit different:  customers need to either enumerate and selectively allow wide Google API IP ranges
or apply the same treament to the traffic as if the workload is within GCP:  send the traffic securely through the VPN Tunnel.

This article is a baseline walkthrough of how to setup an `strongswan` ipsec tunnel to Google and then access GCP APIs and your VMs residing on Google.

[VPC Service Controls](https://cloud.google.com/vpc-service-controls/) are also enabled to demonstrate how to lock down specific API access to just authorized projects.  That is, once the private API is enabled, you can optionally lock down access to APIs like Google Cloud Storage such that it is only accessible via the tunnel.  This is an optional security measure you can employ to further restrict access.

---

## Architecture

The following diagram summarizes the overall architecture that you create in this tutorial.

Connect the two networks through a VPN tunnel and set the routing to resolve and emit GCP APIs through that tunnel. 

* You connect a private network in Amazon VPC to a virtual network in your GCP project through IPsec VPN. If you use an on-premises private network instead of Amazon VPC, you would use Cloud Interconnect to have a private network connection to your GCP project.

* You use Private Google Access from GCP projects. Servers running outside GCP projects cannot reach GCP APIs, such as the Cloud Translation, by using an internal IP address, even when Private Google Access is enabled. 

Notes:

- This article does *not* use BGP dynamic routing as provided by [Cloud Router](https://cloud.google.com/router/docs/).  It instead utilizes setting up [routes](https://cloud.google.com/vpc/docs/routes#instancerouting) on both ends of the system as a demonstration of the low-level routing and `ipsec` traffic management.  For BGP based routing, see [How to set up a VPN between strongSwan and Cloud VPN](https://cloud.google.com/community/tutorials/using-cloud-vpn-with-strongswan).

- This article another GCP Project to "simulate" the remote network. The choice of low-level `ipsec` protocol terminating directly on the 'simulated remote' network demonstrates baseline connectivity that could be used in other environments.  Alternatively, you can setup a tunnel on any provider such as [AWS](https://cloud.google.com/files/CloudVPNGuide-UsingCloudVPNwithAmazonWebServices.pdf).

- Remote nework VPN endpoint is a VM running [openswan](https://www.openswan.org/) for ipsec and [bind](https://en.wikipedia.org/wiki/BIND) for DNS.


![images/private_access.svg](images/private_access.svg)

## Objectives

* Enable Private Google Access in order to allow Remote VMs instances to access Google Cloud Platform APIs without using public IP addresses.

* Optionally enable VPC Service Controls to limit access to GCP APis unless they originate from within a VPC Security perimeter

## Costs

This tutorial uses the following billable components of Google Cloud Platform:

* Compute Engine
* Cloud VPN
* Google Cloud Storage

## Project Structure/Specifications

In the tutorial, we will setup the following remote an local networks.

* Local: 
  - zone: ```us-central1-a```
  - network:```192.168.0.0/20```
* GCP: 
  - zone: ```us-central1-a```
  - network: ```10.10.0.0/20```


  - "OnPrem" simualted network project:  projectId=```ONPREM_PROJECT```
  - GCP network project:  projectId=```GCP_PROJECT```

The steps outlined below sets up the following (in order):

- 1a. [GCP.1]
  - 1b  [GCP] Setup GCP project
  - 1c. [GCP] Setup custom network
  - 1d. [GCP] Create routes for private GCP API access via tunnel
  - 1e. [GCP] Create GCE instance for testing

- 2a. [Remote.1]
  - 2b. [Remote] Setup "onprem" project
  - 2c. [Remote] Setup custom network
  - 2d. [Remote] Set firewall rules for  VPN traffic/gateway and private API access
  - 2e. [Remote] Create VM instance for VPN Gateway
  - 2f. [Remote] Add next-hop route thorugh VPN Gateway  
  - 2g. [Remote] Create VM for testing

- 3a. [GCP.2]
  - 3b. [GCP] Create VPN definition
  - 3c. [GCP] Allow traffic through VPN

- 4a. [Remote.2]
  - 4b. [Remote] Configure VPN gateway VM
  - 4c. [Remote] iptables
  - 4d. [Remote] Strongswan (ipsec)
  - 4e. [Remote] bind (dns)
  - 4f. [Remote] Verify connectivity to VM on [GCP]
  - 4g. [Remote] Verify DNS and route connectivity
  - 4h. [Remote] Verify connectivity from internal VM via gateway

- 5a. [GCP.3] Configure VPC Service Controls


## 1a. Configure GCP.1

This section details setting up the project on GCP. 

It is advised to run all these commands in the same shell to avoid resetting environment variables.

First export some variables (feel free to specify your own projectIDs, ofcourse):

### 1b. [GCP] Setup GCP project

Create project that will host the VPN gateway and VPC API access (substitute the projectID with your own; this article uses `GCP_PROJECT_NAME` in the examples below)

Export the envionment variables for the project and network region to setup

```bash
  export GCP_PROJECT=GCP_PROJECT_NAME
  export GCP_ZONE=us-central1-a
  export GCP_REGION=us-central1
```

### 1c. [GCP] Setup custom network

Create the custom network within this project that will host the VPC and private API access:

```bash
  gcloud compute --project=$GCP_PROJECT networks create private-vpc --subnet-mode=custom

  gcloud compute --project=$GCP_PROJECT networks subnets create private-network --network=private-vpc \
    --region=$GCP_REGION --range=10.10.0.0/20 --enable-private-ip-google-access
```

![images/gcp-project-network.png](images/gcp-project-network.png)

### 1d. [GCP] Create routes for private GCP API access via tunnel

  - Create route for traffic through to ```199.36.153.4/30```

```bash
gcloud --project=$GCP_PROJECT  compute routes create apis --network=private-vpc \
  --destination-range=199.36.153.4/30  --next-hop-gateway=default-internet-gateway
```

![images/gcp-project-private-apis.png](images/gcp-project-private-apis.png)

### 1e. [GCP] Create GCE instance for testing

This VM is used for testing connectivity from the remote VM to GCP.  This VM will not have a public IP allocated and is accessible via VPC>

```bash
gcloud compute --project=$GCP_PROJECT instances create instance-1 \
  --zone=$GCP_ZONE --machine-type=f1-micro --subnet=private-network \
  --no-address --can-ip-forward --no-service-account --no-scopes \
  --image=debian-9-stretch-v20180401 --image-project=debian-cloud \
  --boot-disk-size=10GB --boot-disk-type=pd-standard --boot-disk-device-name=instance-1
```

Verify no public address is allocated:

```bash
$ gcloud compute instances list --project $GCP_PROJECT
    NAME        ZONE           MACHINE_TYPE  PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP  STATUS
    instance-1  us-central1-a  f1-micro                   10.10.0.2                 RUNNING
```

![images/gcp-project-vms.png](images/gcp-project-vms.png)

---

## 2a, Configure Remote.1

### 2b. [Remote] Setup onprem project

This example sets up another GCP project to 'simulate' a remote network.  This project does NOT use Cloud VPN and instead terminates the VPN traffic directly on a VM.  In reality, you can setup any cloud provider VPN or appliance while this article shows raw, low-level `ipsec` configuration

As before, the following command uses a project called `ONPREM_PROJECT`.  In your case, you should setup this project as (for example): `ONPREM_PROJECT-[randomcharacters]`

```
gcloud projects create ONPREM_PROJECT --name=onpremProject
```

Export some environment variables and substitue the variables with your project names:

```bash
  export ONPREM_PROJECT=ONPREM_PROJECT
  export ONPREM_ZONE=us-central1-a
  export ONPREM_REGION=us-central
```

### 2c. [Remote] Setup custom network

Configure a new custom network with a specific CIDR range.  You can configure any range but for consistency with the ipsec configuration below, the CIDR ranges below are used.

```bash
gcloud compute --project=$ONPREM_PROJECT networks create my-network --subnet-mode=custom

gcloud compute --project=$ONPREM_PROJECT networks subnets create my-subnet \
    --network=my-network --region=us-central1 --range=192.168.0.0/20
```

![images/your_vpn_vpc.png](images/your_vpn_vpc.png)

### 2d. [Remote] Set firewall rules for VPN traffic/gateway and private API access

Configure firewall rules on the 'simulated' network to allow `ipsec` traffic inbound and internal traffic to the privateIP ranges for GCP.

```bash
gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-ipsec-500 \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW --rules=udp:500 --source-ranges=0.0.0.0/0

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-ssh-vpn  \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW --rules=tcp:22 --source-ranges=0.0.0.0/0

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-icmp-my-network \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW --rules=icmp --source-ranges=192.168.0.0/20

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-dns-my-network \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW \
  --rules=udp:53 --source-ranges=192.168.0.0/20

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-443-my-network --direction=INGRESS \
  --priority=1000 --network=my-network --action=ALLOW --rules=tcp:443 --source-ranges=192.168.0.0/20
```

![images/your_vpn_firewalls.png](images/your_vpn_firewalls.png)

### 2e. [Remote] Create VM instance for VPN Gateway

This VM will host the `ipsec` tunnel and provide DNS resolution for the `onprem` network.

```bash
gcloud compute --project=$ONPREM_PROJECT instances create instance-1  --zone=$ONPREM_ZONE --machine-type=n1-standard-1 \
  --subnet=my-subnet  --can-ip-forward --no-service-account --no-scopes  --image=debian-9-stretch-v20180401  \
  --image-project=debian-cloud --boot-disk-size=10GB --boot-disk-type=pd-standard --boot-disk-device-name=instance-1
```

### 2f. [Remote] Add next-hop route thorugh VPN Gateway

Since our `onprem` network is yet another GCP project, setup a route via `gcloud`
```bash
gcloud compute --project=$ONPREM_PROJECT routes create route-to-gcp-apis --network=my-network --priority=1000 \
  --destination-range=199.36.153.4/30 --next-hop-instance=instance-1 --next-hop-instance-zone=us-central1-a
```

Normally, you would use BGP or directly set routetables but on GCP, these routes need to get setup on control plane.


### 2g. [Remote] Create VM for testing

Setup another VM instance on the `onprem` network that will send traffic and DNS resolution through the VPN Gateway (`instance-1`)

```bash
gcloud compute --project=$ONPREM_PROJECT instances create instance-2 --zone=$ONPREM_ZONE --machine-type=n1-standard-1 \
  --subnet=my-subnet --can-ip-forward --no-service-account --no-scopes  --image=debian-9-stretch-v20180401 \
  --image-project=debian-cloud --boot-disk-size=10GB --boot-disk-type=pd-standard --boot-disk-device-name=instance-2 
```

In the end, you should have two VMs setup on your `onprem` project:

```bash
$ gcloud compute instances list --project $ONPREM_PROJECT
NAME        ZONE           MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP     STATUS
instance-1  us-central1-a  n1-standard-1               192.168.0.2  35.192.118.145  RUNNING
instance-2  us-central1-a  n1-standard-1               192.168.0.3  35.192.73.171   RUNNING
```

> NOTE public_IP of ONPREM  VPN Gateway) is:  **>>>>  35.192.118.145   <<<<**

```bash
export ONPREM_VPN_IP=35.192.118.145
```

![images/your_vpn_vms.png](images/your_vpn_vms.png)


## 3a. Configure GCP.2

### 3b. [GCP] Create VPN definition

Create VPN using Cloud Console.  

- While setting up the tunnel, select to create a new IP address. 

Use the following specifications for the VPN.  The remote peer IP is the IP address of the VPN host VM we setup previously `35.192.118.145`

Use a new shared secret (eg: `$SHARED_SECRET` = `python -c "import uuid;  print str(uuid.uuid4())"`)

- Name: ```vpn-1```
- Network: ```private-vpc```
- Region: value of ```$ONPREM_REGION```
- IP Address: [select reserve IP for GCP VPN; note the IP address allocated]
- Tunnels
  - Name: vpn-1-tunnel-1
  - Remote peer ip address: value of ```$ONPREM_VPN_IP```
  - Shared secret:  ```TESTSECRET1#```
  - Remote network ip reanges: value of ```192.168.0.0/20```
- Remote Peer IP Address: value of ```$ONPREM_VPN_IP``` which in the screenshot is ```35.192.118.145```

> NOTE public_ip of GCP VPN GATEWAY that gets allocated.  In this article, its: ```35.184.203.133```


![images/gcp-project_vpn.png](images/gcp-project_vpn.png)

> NOTE THE VPN GATEWAY PUBLIC IP   **>>>>  35.184.203.133  <<<<***

export the ip as an environment variable for later use

```bash
export GCP_VPN_IP=35.184.203.133
```

### 3c. [GCP] Allow traffic through VPN

On the GCP network, allow traffic inbound from your `onprem` network range `192.168.0.0/20`

```bash
gcloud compute --project=$GCP_PROJECT firewall-rules create allow-vpn-traffic --direction=INGRESS \
  --priority=1000 --network=private-vpc --action=ALLOW --rules=tcp,udp,icmp --source-ranges=192.168.0.0/20
```

## 4a. Configure Remote.2

### 4b. [Remote] Configure VPN gateway VM

Configure the "ONPREM" VPN gateway

```
gcloud compute ssh instance-1 --project=$ONPREM_PROJECT
```
  > Remember to replace ```leftid=``` with the IP address of the VPN Gateway VM on ```$ONPREM_VPN_IP```

  > Remember to replace ```right=``` with the IP address of the VPN Gateway VM on ```$GCP_VPN_IP```

then

```bash
  $ sudo su -
  $ apt-get update && apt-get install -y strongswan iptables ipsec-tools dnsutils traceroute bind9 vim telnet
```

#### 4c. [Remote] iptables

- Create iptable rules to allow forwarding and tunnel
  - ```/etc/rules.v4```

```
*nat
:PREROUTING ACCEPT  [0:0]
:POSTROUTING ACCEPT  [0:0]

-A POSTROUTING -o eth0 -j MASQUERADE
-A POSTROUTING ! -d 10.10.0.0/20 -o eth0 -j MASQUERADE
-A POSTROUTING ! -d 199.36.153.4/30 -o eth0 -j MASQUERADE
COMMIT

*filter
:INPUT ACCEPT [0:0]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT  [0:0]

-A INPUT  -j ACCEPT

-A OUTPUT -j ACCEPT
COMMIT
```

- Apply firewall rules

```
iptables-restore <  /etc/rules.v4
```

- Set system to allow packet forwarding

```bash
sysctl -w net.ipv4.ip_forward=1
echo 1 > /proc/sys/net/ipv4/ip_forward
```

#### 4d. [Remote] Strongswan (ipsec) configuration

Configure strongswan secrets:

- Create ```/etc/ipsec.secrets```
  - Add

```
GCP_VPN_IP ONPREM_VPN_PUBLIC_IP :   PSK "TESTSECRET1#"
```

In the current example, the values would be:

```
35.184.203.133  35.192.118.145 :   PSK "TESTSECRET1#"
```

- Create `/etc/ipsec.conf`

Configure `ipsec.conf`:

  > Remember to replace ```leftid=``` with the IP address of the VPN Gateway VM on ```$ONPREM_VPN_IP```

  > Remember to replace ```right=``` with the IP address of the VPN Gateway VM on ```$GCP_VPN_IP```


``` 
version 2.0
config setup
        protostack=netkey
        plutodebug="control dpd"
        plutostderrlog=/var/log/pluto.log

conn site-to-site
        authby=secret
        type=tunnel

        ike=aes256-sha1-modp2048
        esp=aes256-sha1-modp2048
        ikelifetime=3h
        lifetime=10h
        rekeymargin=3m
        keyingtries=%forever
        authby=secret
        dpddelay=15
        dpdtimeout=60
        dpdaction=restart
        auto=start
        keyexchange=ikev2
 
        left=192.168.0.2   
        leftid=ONPREM_VPN_PUBLIC_IP
        leftsubnet=192.168.0.0/20

        right=GCP_VPN_IP
        rightsubnet=10.10.0.0/20,199.36.153.4/30

include /var/lib/strongswan/ipsec.conf.inc
```

- Restart ipsec 

Verify tunnels are created

```bash
root@instance-1:~# service ipsec restart

root@instance-1:~# service ipsec status
● strongswan.service - strongSwan IPsec IKEv1/IKEv2 daemon using ipsec.conf
   Loaded: loaded (/lib/systemd/system/strongswan.service; enabled; vendor preset: enabled)
   Active: active (running) since Sun 2019-03-17 01:11:56 UTC; 3s ago
 Main PID: 8886 (starter)
    Tasks: 18 (limit: 4915)
   CGroup: /system.slice/strongswan.service
           ├─8886 /usr/lib/ipsec/starter --daemon charon --nofork
           └─8900 /usr/lib/ipsec/charon

Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] authentication of '35.184.203.133' with pre-shared key successful
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] authentication of '35.192.118.145' (myself) with pre-shared key
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] IKE_SA site-to-site[3] established between 192.168.0.2[35.192.118.145]...35.184.203.133[35.184.203.133]
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] IKE_SA site-to-site[3] established between 192.168.0.2[35.192.118.145]...35.184.203.133[35.184.203.133]
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] scheduling reauthentication in 10535s
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] maximum IKE_SA lifetime 10715s
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] CHILD_SA site-to-site{2} established with SPIs c5349dc6_i a05b0780_o and TS 192.168.0.0/20 === 10.10.0.0/20 199.36.153.4/30
Mar 17 01:11:57 instance-1 charon[8900]: 16[IKE] CHILD_SA site-to-site{2} established with SPIs c5349dc6_i a05b0780_o and TS 192.168.0.0/20 === 10.10.0.0/20 199.36.153.4/30
Mar 17 01:11:57 instance-1 charon[8900]: 16[ENC] generating IKE_AUTH response 1 [ IDr AUTH SA TSi TSr N(AUTH_LFT) ]
Mar 17 01:11:57 instance-1 charon[8900]: 16[NET] sending packet: from 192.168.0.2[4500] to 35.184.203.133[4500] (236 bytes)
```
2f. [Remote] Add next-hop route thorugh VPN Gateway

Verify the tunnels are up:

```bash
root@instance-1:~# ip xfrm state
src 192.168.0.2 dst 35.184.203.133
	proto esp spi 0x2f09cd79 reqid 1 mode tunnel
	replay-window 0 flag af-unspec
	auth-trunc hmac(sha1) 0xc2ee36f1aa6d335d015548ef05faf1224e69cb91 96
	enc cbc(aes) 0xf07963ea41e43e968ed664f238b733ac09c073fc875eaab7bac42a38d3c2cd84
	encap type espinudp sport 4500 dport 4500 addr 0.0.0.0
	anti-replay context: seq 0x0, oseq 0x1a, bitmap 0x00000000
src 35.184.203.133 dst 192.168.0.2
	proto esp spi 0xc540db4a reqid 1 mode tunnel
	replay-window 32 flag af-unspec
	auth-trunc hmac(sha1) 0x7349e75f2099d135e2afbe1c1822e914bc6fa367 96
	enc cbc(aes) 0xc6847b37afef24fc618a09e4cd8a86e004176373b75328413342adb050fba1c1
	encap type espinudp sport 4500 dport 4500 addr 0.0.0.0
	anti-replay context: seq 0x14, oseq 0x0, bitmap 0x000fffff
```

- Verify connectivity to GCP VM's private IP

```bash
root@instance-1:~# ping 10.10.0.2 
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=64 time=2.76 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=64 time=1.21 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=64 time=1.24 ms
64 bytes from 10.10.0.2: icmp_seq=4 ttl=64 time=1.26 ms
```

Open up a new window in `instance-1`, and run `ip xfrm monitor`.

You should see ipsec traffic through the gateway vm `instance-1`

```bash
root@instance-1:~# ip xfrm monitor
Async event  (0x20)  timer expired 
	src 192.168.0.2 dst 35.184.203.133  reqid 0x1 protocol esp  SPI 0xa8664240
Async event  (0x20)  timer expired 
	src 35.184.203.133 dst 192.168.0.2  reqid 0x1 protocol esp  SPI 0xc5d98368
Async event  (0x20)  timer expired 
	src 192.168.0.2 dst 35.184.203.133  reqid 0x1 protocol esp  SPI 0xa8664240
Async event  (0x20)  timer expired 
	src 35.184.203.133 dst 192.168.0.2  reqid 0x1 protocol esp  SPI 0xc5d98368
```

#### 4e. [Remote] bind (DNS) configuration

Private access for Google APIs from remote systems requries a special ```CNAME``` map: 

- Normally ```www.googleapis.com``` resolves for IP addresses that are external but over VPN, a specific CNAME map as such is required:

  * ```www.googlapis.com:  CNAME=restricted.googleapis.com``` --> ```199.36.153.4/30```
  * the IP reange ```199.36.153.4/30``` is routed though the tunnel and will handle only internal traffic within GCP as Private Acccess


- Setup DNS CNAMES for ```restricted.googleapis.com```

  On remote gateway VM (`instance-1`):

  - Edit/Create ```/etc/bind/named.conf.local```

``` 
//include "/etc/bind/zones.rfc1918";
//include "/etc/bind/named.conf.default-zones";

zone "googleapis.zone" {
  type master;
  file "/etc/bind/db.googleapis.zone";
  allow-query {none;};
};
```

- Edit/Create ```/etc/bind/named.conf.options```

```
options {
  directory "/var/cache/bind";

   forwarders {
    169.254.169.254;
   };

   allow-query { any;};
   response-policy { zone "googleapis.zone"; };

  dnssec-validation auto;

  auth-nxdomain no;    # conform to RFC1035
  listen-on-v6 { any; };
};
```

- Create/Edit ```/etc/bind/named.conf```

```
include "/etc/bind/named.conf.options";
include "/etc/bind/named.conf.local";
include "/etc/bind/named.conf.default-zones";
```

- Create/Edit ```/etc/bind/db.googleapis.zone```

```
$TTL 1H
@                       SOA LOCALHOST. noreply.localhost(1 1h 15m 30d 2h)
                        NS  LOCALHOST.

*.googleapis.com CNAME restricted.googleapis.com.
restricted.googleapis.com CNAME rpz-passthru.
```

- Restart dns/bind
```bash
service bind9 restart
```

### 4f. [Remote] Verify connectivity to VM and GCP APIs [GCP]

  On remote gateway VM (`instance-1`), open up two new shells.

  In one window, run `ip xfrm monitor`,  In another window, access the GCE VM and a private API:

```bash
root@instance-1::~$ ping 10.10.0.2
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=64 time=1.69 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=64 time=0.891 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=64 time=0.954 ms
```

Also verify the traffic through to `199.36.153.7` is via the tunnel
```bash
root@instance-1:~# telnet 199.36.153.7 443
Trying 199.36.153.7...
Connected to 199.36.153.7.
Escape character is '^]'.
```

The session with `ip xfrm monitor` shows activity which indicates data sent via tunnel.

### 4g. [Remote] Verify DNS and route connectivy

The following sequece will configure and test DNS resolution and direct connectivity to:

  - ```restricted.googleapis.com```
  - ```www.googleapis.com```

  On remote gateway VM (`instance-1`):

- Set DNS server to resovle address locally first (add `nameserver 127.0.0.1` as first nameserver entry:
  - Create/Edit- ```/etc/resolv.conf```

```
domain c.ONPREM_PROJECT.internal
search c.ONPREM_PROJECT.internal. google.internal.
nameserver 127.0.0.1
nameserver 169.254.169.254
```
> Note `ONPREM_PROJECT`, the settings in your file will be different.

>> **NOTE:** GCP VMs overrides certain host entries likes `/etc/resolv.conf` so you may need to reset this if you leave the 'onprem' VMs running.

- Now lookup ```www.googleapis.com```

```bash
$ nslookup www.googleapis.com
Server:   127.0.0.1
Address:  127.0.0.1#53

Non-authoritative answer:
www.googleapis.com  canonical name = restricted.googleapis.com.
Name: restricted.googleapis.com
Address: 199.36.153.6
Name: restricted.googleapis.com
Address: 199.36.153.5
Name: restricted.googleapis.com
Address: 199.36.153.4
Name: restricted.googleapis.com
Address: 199.36.153.7
```

This should resolve to the IPs provided locally with CNAME and resolve to the `199.36.153.` range


- Connect to a google APIs

  - Acquire an access_token _from your laptop_ ```gcloud auth print-access-token```

  On remote gateway VM (`instance-1`), try to access a GCS endpoint.  (note, replace `?project=` value in the URL with your `$GCP_PROJECT`)

```bash
# curl -vvvv -H "Authorization: Bearer <ACCESS_TOKEN>" https://www.googleapis.com/storage/v1/b?project=$GCP_PROJECT

* Connected to www.googleapis.com (199.36.153.5) port 443 (#0)

< HTTP/2 200 
 
{
 "kind": "storage#buckets",
 "items": [
  {
   "kind": "storage#bucket",
```

> Note the IP address you connected to: ```199.36.153.5```

At this point, we have verified GCS API calls through the tunnel

### 4h. [Remote] Verify connectivity from internal VM via gateway

We have verified the remote gateway sends traffic to GCP via the tunnel.  We will now configure another host 'on prem' which will
also send its traffic through to GCP via this gateway:

 - Connecting from intenral VM (`instance-2` --> GCP via gateway (`instance-1`)

Normally, routes are added directly if using BGP or static route:

```bash
ip route add 10.10.0.0/20 via 192.168.0.2
```

However, since this tutorial simulates 'onprem' on GCP, add routes via `gcloud` instead:

```bash
gcloud compute --project=$ONPREM_PROJECT routes create route-to-gcp  --network=my-network --priority=1000   --destination-range=10.10.0.0/20  --next-hop-instance=instance-1 --next-hop-instance-zone=us-central1-a
```

- Now from ```instance-2```, connect to the remote VM on GCP through the VPN gateway VM:

```bash
root@instance-2:~# ping 10.10.0.2
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=63 time=3.88 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=63 time=1.15 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=63 time=1.20 ms
```

- Set DNS Resolution to look for the VPN-gateway's `bind9` server.  In this case, the VPN gateway's ip is `192.168.0.2`

```bash
root@instance-2:~# more /etc/resolv.conf 
domain c.ONPREM_PROJECT.internal
search c.ONPREM_PROJECT.internal. google.internal.
nameserver 192.168.0.2
nameserver 169.254.169.254
```

- Access GCS and veirfy the IP connected is the restricted range (eg, in this example, its ```199.36.153.7```)

```bash
root@instance-2::~$ curl -vvvv -H "Authorization: Bearer <ACCESS_TOKEN>" \ 
      https://www.googleapis.com/storage/v1/b?project=YOUR_PROJECT

* Connected to www.googleapis.com (199.36.153.7) port 443 (#0)

< HTTP/2 200 

{
 "kind": "storage#buckets",
 "items": [
  {
   "kind": "storage#bucket",
```

Verify API traffic is through the tunnel by running `ip xfrm monitor` on the VPN Gateway host (eg, `instance-1` on `project=ONPREM_PROJECT_NAME`)

```bash
root@instance-1:~# ip xfrm monitor
Async event  (0x20)  timer expired 
	src 192.168.0.2 dst 35.184.203.133  reqid 0x1 protocol esp  SPI 0xa8664240
Async event  (0x20)  timer expired 
	src 35.184.203.133 dst 192.168.0.2  reqid 0x1 protocol esp  SPI 0xc5d98368
Async event  (0x20)  timer expired 
	src 192.168.0.2 dst 35.184.203.133  reqid 0x1 protocol esp  SPI 0xa8664240
Async event  (0x20)  timer expired 
	src 35.184.203.133 dst 192.168.0.2  reqid 0x1 protocol esp  SPI 0xc5d98368
```


### 5a. [GCP.3] VPC Service Control

At this point, the VPN connectivity to GCP APIs and VM is established via the tunnel.  Optionally enable API access to GCP services such that it must go through a trusted project (in our case, via the tunnel).

See [Service Perimeters](https://cloud.google.com/vpc-service-controls/docs/create-service-perimeters) as well as [Cloud IAM Roles for Administering VPC Service Controls](https://cloud.google.com/vpc-service-controls/docs/access-control).

First enable a security perimeter such that GCS access is only available via `project=gcp-project`:

![images/service_perimeter.png](images/service_perimeter.png)

> Note: once you enable this, all acess to GCS for the given bucket is blocked except via this specific project.


on a host 'on prem' force traffic for `www.googleapis.com` to _not_ go through the tunnel. Do this buy making the DNS resolve to the external address.  On `instance-2`, comment the name resolution entry:

```bash
root@instance-2:~# more /etc/resolv.conf 
domain c.ONPREM_PROJECT.internal
search c.ONPREM_PROJECT.internal. google.internal.
#nameserver 192.168.0.2
nameserver 169.254.169.254
```

Then attempt to access GCS:  

(note the address is outside of the tunnel route: `74.125.70.95`)

```bash
root@instance-2:~# curl -vvvv -H "Authorization: Bearer <ACCESS_TOKEN>" https://www.googleapis.com/storage/v1/b?project=GCP_PROJECT_NAME
*   Trying 74.125.70.95...
* TCP_NODELAY set
* Connected to www.googleapis.com (74.125.70.95) port 443 (#0)


< HTTP/2 403 

{
 "error": {
  "errors": [
   {
    "domain": "global",
    "reason": "vpcServiceControls",
    "message": "Request violates VPC Service Controls."
   }
  ],
  "code": 403,
  "message": "Request violates VPC Service Controls."
 }
}

```

Now add in name resolution such that the traffic transits the gateway and tunnel:

(note the address resolved is `199.36.153.4`)

```bash
root@instance-2:~# more /etc/resolv.conf 
domain c.ONPREM_PROJECT.internal
search c.ONPREM_PROJECT.internal. google.internal.
nameserver 192.168.0.2
nameserver 169.254.169.254
```

```bash
root@instance-2:~# curl -vvvv -H "Authorization: Bearer <ACCESS_TOKEN>" https://www.googleapis.com/storage/v1/b?project=GCP_PROJECT_NAME
*   Trying 199.36.153.4...

< HTTP/2 200
{
 "kind": "storage#buckets",
```

# Cleaning up

When you have completed this tutorial, delete your project to avoid incurring further costs.

---

# References

- [Configuring Private Google Access for on-premises hosts](https://cloud.google.com/vpc/docs/configure-private-google-access-hybrid)
- [Using APIs from an External Network](https://cloud.google.com/solutions/using-gcp-apis-from-an-external-network)
- [How to set up a VPN between strongSwan and Cloud VPN](https://cloud.google.com/community/tutorials/using-cloud-vpn-with-strongswan)
- [Configuring Private Google Access](https://cloud.google.com/vpc/docs/configure-private-google-access)
