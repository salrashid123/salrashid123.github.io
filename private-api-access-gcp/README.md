# Private Access to GCP APIs through VPN Tunnels

GCP customers often have workloads spanning cloud proividers and their on-premises datacenters connected via VPN.  In many of those 
situations, customers need to access various Google Cloud APIs where arbitrary outbound traffic from
any system is restricted or acutely controlled.  

This not a problem while the workload is on GCP:  accessing GCP APIs from within is directed
towards internal interfaces where those services resides.  When accessing these same APIs from other restricted cloud providers or 
even your own datacenter the situation is a bit different:  you need to either enumerate and selectively allow wide Google API IP ranges
or apply the same treament to the traffic as if the workload is within GCP:  send the traffic securely through the VPN Tunnel.

[VPC Service Controls](https://cloud.google.com/vpc-service-controls/) are also enabled to demonstrate how to lock down specific API access to just authorized projects.  That is, once the private API is enabled, you can optionally lock down access to Google Cloud Storage APIs such that it is only accessible via the tunnel.  This is an optional security measure you can employ.

This article is a walkthough on how to setup GCP API access from your remote system through a VPN tunnel.

---

## Architecture

The steup is relatively simple:  connect the networks through a VPN tunnel and set the routing to resolve and emit GCP APIs through it. 

Some points to note:

- This article does not use BGP dynamic routing as provided by [Cloud Router](https://cloud.google.com/router/docs/).  It instead utilizes setting up [routes](https://cloud.google.com/vpc/docs/routes#instancerouting) on both ends of the system.
- Uses another GCP Project to "simulate" the remote network.
- Remote nework VPN endpoint is a VM running [openswan](https://www.openswan.org/) for ipsec and [bind](https://en.wikipedia.org/wiki/BIND) for DNS.

Private access for Google APIs from remote systems requries a special ```CNAME``` map: 
- Normally ```www.googleapis.com``` resolves for IP addresses that are external. but  over VPN, a specific CNAME map as such is required:
  * ```www.googlapis.com:  CNAME=restricted.googleapis.com``` --> ```199.36.153.4/30```
  * the IP reange ```199.36.153.4/30``` is routed though the tunnel and will handle only internal traffic within GCP as Private Acccess

![images/private_access.svg](images/private_access.svg)


## Project Structure/Specifications

Lets get started.  In the example below specifies the local/remote networks as such:

* Local: 
  - project:  ```your-vpn```
  - zone: ```us-central1-a```
  - network:```192.168.0.0/20```
* GCP: 
  - project:  ```gcp-project-200601```
  - zone: ```us-central1-a```
  - network: ```10.10.0.0/20```

The steps outlined below sets up the following (in order):

- [GCP] setup project
- [GCP] setup custom network
- [GCP] create routes for private GCP API access via tunnel
- [GCP] create GCE instance for testing
- [Remote] setup project
- [Remote] setup custom network
- [Remote] set firewall rules for  VPN traffic/gateway and private API access
- [Remote] create VM instance for VPN Gateway
- [Remote] create VM for testing
- [GCP] create VPN definition
- [GCP] allow traffic through VPN
- [Remote] configure VPN gateway VM
  - [Remote] iptables
  - [Remote] strongswan (ipsec)
  - [Remote] bind (dns)
- [Remote] verify connectivity to VM on [GCP]
- [Remote] verify DNS and route connectivy to 
  - ```restricted.googleapis.com```
  - ```www.googleapis.com```
- [Remote] verify connectivity from internal VM via gateway
- [GCP] Configure VPC Service Controls


## Configure GCP.1

This seciton details setting up the project on GCP. 

It is advised to run all these commands in the same shell to avoid resetting environment variables.

First export some variables (feel free to specify your own projectIDs, ofcourse):

### [GCP] setup custom network

```
  export GCP_PROJECT=gcp-project-200601
  export GCP_ZONE=us-central1-a
  export GCP_REGION=us-central
```

### [GCP] setup custom network

```
  gcloud compute --project=$GCP_PROJECT networks create private-vpc --mode=custom

  gcloud compute --project=$GCP_PROJECT networks subnets create private-network \ 
     --network=private-vpc --region=$GCP_REGION --range=10.10.0.0/20 \ 
     --enable-private-ip-google-access
```

![images/gcp-project-network.png](images/gcp-project-network.png)

### [GCP] create routes for private GCP API access via tunnel
  - Create route for traffic through to  ```199.36.153.4/30```

```
gcloud --project=$GCP_PROJECT  compute routes create apis --network=private-vpc \
  --destination-range=199.36.153.4/30  --next-hop-gateway=default-internet-gateway
```

![images/gcp-project-private-apis.png](images/gcp-project-private-apis.png)

### [GCP] create GCE instance for testing

```
gcloud compute --project=GCP_PROJECT instances create instance-1 \
  --zone=$GCP_ZONE --machine-type=f1-micro --subnet=private-network \
  --no-address --can-ip-forward --no-service-account --no-scopes \
  --image=debian-9-stretch-v20180401 --image-project=debian-cloud \
  --boot-disk-size=10GB --boot-disk-type=pd-standard --boot-disk-device-name=instance-1
```

```  
$ gcloud compute instances list --project $GCP_PROJECT
    NAME        ZONE           MACHINE_TYPE  PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP  STATUS
    instance-1  us-central1-a  f1-micro                   10.10.0.2                 RUNNING
```

![images/gcp-project-vms.png](images/gcp-project-vms.png)

---

## Configure Remote.1

Setup the environment variables (as before, use your own project)

### [Remote] setup project
```
  export ONPREM_PROJECT=your-vpn
  export ONPREM_ZONE=us-central1-a
  export ONPREM_REGION=us-central
```

### [Remote] setup custom network

```  
gcloud compute --project=$ONPREM_PROJECT networks create my-network --mode=custom

gcloud compute --project=$ONPREM_PROJECT networks subnets create my-subnet \
    --network=my-network --region=us-central1 --range=192.168.0.0/20
```

![images/your_vpn_vpc.png](images/your_vpn_vpc.png)

### [Remote] set firewall rules for VPN traffic/gateway and private API access

```
gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-ipsec-500 \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW \
  --rules=udp:500 --source-ranges=0.0.0.0/0

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-ssh-vpn  \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW \ 
  --rules=tcp:22 --source-ranges=0.0.0.0/0

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-icmp-my-network \ 
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW  \
  --rules=icmp --source-ranges=192.168.0.0/20

gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-dns-my-network \
  --direction=INGRESS --priority=1000 --network=my-network --action=ALLOW  \ 
  --rules=udp:53 --source-ranges=192.168.0.0/20

gcloud compute --project=your-vpn firewall-rules create allow-443-my-network  \ 
  --direction=EGRESS --priority=1000 --network=my-network --action=ALLOW \ 
  --rules=tcp:443 --destination-ranges=192.168.0.0/20
```

![images/your_vpn_firewalls.png](images/your_vpn_firewalls.png)

### [Remote] create VM instance for VPN Gateway

```
gcloud compute --project=$ONPREM_PROJECT instances create instance-1 \ 
  --zone=$ONPREM_ZONE --machine-type=n1-standard-1 --subnet=my-subnet \ 
  --can-ip-forward --no-service-account --no-scopes  --image=debian-9-stretch-v20180401  \ 
  --image-project=debian-cloud --boot-disk-size=10GB --boot-disk-type=pd-standard \ 
 --boot-disk-device-name=instance-1
```

### [Remote] Add next-hop route thorugh VPN Gateway

```
gcloud compute --project=your-vpn routes create route-to-gcp-apis \ 
  --network=my-network --priority=1000 --destination-range=199.36.153.4/30 \ 
  --next-hop-instance=instance-1 --next-hop-instance-zone=us-central1-a
```

### [Remote] create VM for testing

```
gcloud compute --project=$ONPREM_PROJECT instances create instance-2 --zone=$ONPREM_ZONE \ 
  --machine-type=n1-standard-1 --subnet=my-subnet --can-ip-forward --no-service-account \ 
  --no-scopes  --image=debian-9-stretch-v20180401 --image-project=debian-cloud \ 
  --boot-disk-size=10GB --boot-disk-type=pd-standard --boot-disk-device-name=instance-2 
```

```
$ gcloud compute instances list --project $ONPREM_PROJECT
NAME        ZONE           MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP     STATUS
instance-1  us-central1-a  n1-standard-1               192.168.0.2  35.192.118.145  RUNNING
instance-2  us-central1-a  n1-standard-1               192.168.0.3  35.192.73.171   RUNNING
```

> NOTE public_IP of ONPREM  VPN Gateway) is:  **>>>>  35.192.118.145   <<<<**

```
export ONPREM_VPN_IP=35.192.118.145
```

![images/your_vpn_vms.png](images/your_vpn_vms.png)


## Configure GCP.2

### [GCP] create VPN definition

Create VPN using GUI.  

- Select to create a new IP address. 
   
> NOTE public_ip of GCP VPN GATEWAY ```35.184.203.133```
```
export GCP_VPN_IP=35.184.203.133
```

Use the following settings:

- name: ```vpn-1```
- network: ```private-vpc```
- region: value of ```$ONPREM_REGION```
- Tunnels
  - name: vpn-1-tunnel-1
  - remote peer ip address: value of ```$ONPREM_VPN_IP```
  - shared secret:  ```VPNSECRET1#```
  - remote network ip reanges: value of ```192.168.0.0/20```
- Remote Peer IP Address: value of ```$ONPREM_VPN_IP``` which in the screenshot is ```35.192.118.145```

![images/gcp-project_vpn.png](images/gcp-project_vpn.png)

> NOTE THE VPN GATEWAY PUBLIC IP   **>>>>  35.184.203.133  <<<<**experimental/users/srashid/misc/vpn_private_access/README.md


### [GCP] allow traffic through VPN

```
gcloud compute --project=$ONPREM_PROJECT firewall-rules create allow-vpn-traffic \ 
  --direction=INGRESS --priority=1000 --network=private-vpc --action=ALLOW \ 
  --rules=tcp,udp,icmp --source-ranges=192.168.0.0/20
```

## Configure Remote.2

### [Remote] configure VPN gateway VM

Configure the "ONPREM" VPN gatewa

```
gcloud compute ssh instance-1 --project=$ONPREM_PROJECT
```

then

```
  $ sudo su -
  $ apt-get update && apt-get install -y strongswan iptables ipsec-tools dnsutils traceroute bind9
```

### [Remote] configure VPN gateway VM

#### [Remote] iptables

- Create ipscec rules to allow forwarding and tunnel
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

```
sysctl -w net.ipv4.ip_forward=1
echo 1 > /proc/sys/net/ipv4/ip_forward
```

#### [Remote] strongswan (ipsec)

- Create ```/etc/ipsec.secrets```
  - Add

```
YOUR_VPN_PUBLIC_IP GCP_VPN_IP :   PSK "VPNSECRET1#"
```

for me its

```
35.184.203.133  35.192.118.145 :   PSK "VPNSECRET1#"
```

- Create ```/etc/ipsec.secrets```

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

        ike=aes256-sha1-modp1536
        esp=aes256-sha1-modp1536
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
        leftid=35.192.118.145
        leftsubnet=192.168.0.0/20

        right=35.184.203.133
        rightsubnet=10.10.0.0/20,199.36.153.4/30

include /var/lib/strongswan/ipsec.conf.inc
```

- Restart ipsec 

Verify tunnels are created

```
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


You can verify the tunnel networks by running:

```
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

and specific policies:

```
root@instance-1:~# ip xfrm policy
src 199.36.153.4/30 dst 192.168.0.0/20 
	dir fwd priority 187200 ptype main 
	tmpl src 35.184.203.133 dst 192.168.0.2
		proto esp reqid 1 mode tunnel
src 199.36.153.4/30 dst 192.168.0.0/20 
	dir in priority 187200 ptype main 
	tmpl src 35.184.203.133 dst 192.168.0.2
		proto esp reqid 1 mode tunnel
src 192.168.0.0/20 dst 199.36.153.4/30 
	dir out priority 187200 ptype main 
	tmpl src 192.168.0.2 dst 35.184.203.133
		proto esp reqid 1 mode tunnel
src 10.10.0.0/20 dst 192.168.0.0/20 
	dir fwd priority 189760 ptype main 
	tmpl src 35.184.203.133 dst 192.168.0.2
		proto esp reqid 1 mode tunnel
src 10.10.0.0/20 dst 192.168.0.0/20 
	dir in priority 189760 ptype main 
	tmpl src 35.184.203.133 dst 192.168.0.2
		proto esp reqid 1 mode tunnel
src 192.168.0.0/20 dst 10.10.0.0/20 
	dir out priority 189760 ptype main 
	tmpl src 192.168.0.2 dst 35.184.203.133
		proto esp reqid 1 mode tunnel
```

- Check if we can ping the remote VM on GCP

```
root@instance-1:~# ping 10.10.0.2 
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=64 time=2.76 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=64 time=1.21 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=64 time=1.24 ms
64 bytes from 10.10.0.2: icmp_seq=4 ttl=64 time=1.26 ms
```

You should see ipsec traffic through the gateway vm `instance-1`

```
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

#### [Remote] bind (dns)
- Setup DNS CNAMES for ```restricted.googleapis.com```

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
```
service bind9 restart
```

### [Remote] verify connectivity to VM on [GCP]

```
root@instance-1::~$ ping 10.10.0.2
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=64 time=1.69 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=64 time=0.891 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=64 time=0.954 ms
```

### [Remote] verify DNS and route connectivy to 
  - ```restricted.googleapis.com```
  - ```www.googleapis.com```

```
root@instance-1:~# nslookup restricted.googleapis.com
Server:   169.254.169.254
Address:  169.254.169.254#53

Non-authoritative answer:
Name: restricted.googleapis.com
Address: 199.36.153.7
Name: restricted.googleapis.com
Address: 199.36.153.6
Name: restricted.googleapis.com
Address: 199.36.153.5
Name: restricted.googleapis.com
Address: 199.36.153.4
```
   The value here are external, this isn't using our local server

  - Lookup ```restricted.googleapis.com``` using local DNS 

```
# nslookup www.googleapis.com 127.0.0.1
Server:   127.0.0.1
Address:  127.0.0.1#53

Non-authoritative answer:
www.googleapis.com  canonical name = restricted.googleapis.com.
Name: restricted.googleapis.com
Address: 199.36.153.4
Name: restricted.googleapis.com
Address: 199.36.153.6
Name: restricted.googleapis.com
Address: 199.36.153.7
Name: restricted.googleapis.com
Address: 199.36.153.5
```

You should see the ```199.``` range

- Verify connectivity to an IP for ```restricted.googleapis.com```

```
root@instance-1:~# telnet 199.36.153.7 443
Trying 199.36.153.7...
Connected to 199.36.153.7.
Escape character is '^]'.
```

- Check lookup for ```www.googleapis.com``` using default GCE DNS

```
root@instance-1:/etc/bind# nslookup www.googleapis.com
Server:   169.254.169.254
Address:  169.254.169.254#53

Non-authoritative answer:
www.googleapis.com  canonical name = googleapis.l.google.com.
Name: googleapis.l.google.com
Address: 108.177.112.95
Name: googleapis.l.google.com
Address: 74.125.70.95
Name: googleapis.l.google.com
Address: 74.125.124.95
Name: googleapis.l.google.com
Address: 173.194.192.95
```

- Check lookup for ```www.googleapis.com``` using local DNS override

```
root@instance-1:/etc/bind# nslookup www.googleapis.com 127.0.0.1
Server:   127.0.0.1
Address:  127.0.0.1#53

Non-authoritative answer:
www.googleapis.com  canonical name = restricted.googleapis.com.
Name: restricted.googleapis.com
Address: 199.36.153.6
Name: restricted.googleapis.com
Address: 199.36.153.5
Name: restricted.googleapis.com
Address: 199.36.153.7
Name: restricted.googleapis.com
Address: 199.36.153.4
```

It should resovle to the same CNAME for ```restricted.googleapis.com```

- Set DNS server to resovle locally:
  - Create/Edit- ```/etc/resolv.conf```

```
domain c.your-vpn.internal
search c.your-vpn.internal. google.internal.
nameserver 127.0.0.1
nameserver 169.254.169.254
```

- Now lookup ```www.googleapis.com```

```
nslookup www.googleapis.com
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

This should resolve to the IPs provided locally with CNAME


- Connect to a google APIs

  - Acquire an access_token from your laptop ```gcloud auth print-access-token```

```
# curl -vvvv -H "Authorization: Bearer ya29.-REDACTED" https://www.googleapis.com/storage/v1/b?project=mineral-minutia-820

* Connected to www.googleapis.com (199.36.153.5) port 443 (#0)

< HTTP/2 200 
 
{
 "kind": "storage#buckets",
 "items": [
  {
   "kind": "storage#bucket",
```

> Note the IP address you connected to: ```199.36.153.5```

### [Remote] verify connectivity from internal VM via gateway

 - Connecting from intenral VM (instance-2 --> GCP via gateway (instance-1)

normally, routes are added directly if using BGP but since we're using static routes, you would have to set them up:

```
ip route add 10.10.0.0/20 via 192.168.0.2
```

however, since our 'onprem' instance is itself on GCP, we need to add routes via `gcloud` instead:

```
gcloud compute --project=$ONPREM_PROJECT routes create route-to-gcp  \
   --network=my-network --priority=1000 --destination-range=10.10.0.0/20 \ 
   --next-hop-instance=instance-1 --next-hop-instance-zone=us-central1-a
```

- Now from ```instance-2```, connect over to the reote VM on GCP through the VPN gateway VM:

```
root@instance-2:~# ping 10.10.0.2
PING 10.10.0.2 (10.10.0.2) 56(84) bytes of data.
64 bytes from 10.10.0.2: icmp_seq=1 ttl=63 time=3.88 ms
64 bytes from 10.10.0.2: icmp_seq=2 ttl=63 time=1.15 ms
64 bytes from 10.10.0.2: icmp_seq=3 ttl=63 time=1.20 ms
```

- Set DNS Resolution to look for the VPN-gateway's `bind9` server

```
root@instance-2:~# more /etc/resolv.conf 
domain c.your-vpn.internal
search c.your-vpn.internal. google.internal.
nameserver 192.168.0.2
nameserver 169.254.169.254
```

- Verify DNS for  ```www.googleapis.com```

```
root@instance-2:~# nslookup www.googleapis.com
Server:   192.168.0.2
Address:  192.168.0.2#53

Non-authoritative answer:
www.googleapis.com  canonical name = restricted.googleapis.com.
Name: restricted.googleapis.com
Address: 199.36.153.6
Name: restricted.googleapis.com
Address: 199.36.153.7
Name: restricted.googleapis.com
Address: 199.36.153.4
Name: restricted.googleapis.com
Address: 199.36.153.5

```

- Access GCS and veirfy the IP connected is our restricted ```199.36.153.7```

```
root@instance-2::~$ curl -vvvv -H "Authorization: Bearer ya29.Gl2bBV-REDACTED" \ 
      https://www.googleapis.com/storage/v1/b?project=mineral-minutia-820 

* Connected to www.googleapis.com (199.36.153.7) port 443 (#0)

< HTTP/2 200 

{
 "kind": "storage#buckets",
 "items": [
  {
   "kind": "storage#bucket",
```

Verify API traffic is through the tunnel by running `ip xfrm monitor` on the VPN Gateway host (eg, `instance-1` on `project=your_vpn`)

```
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


### VPC Service Control

Now that we've established API access via the tunnel, you can optionally enable API access to GCP services such that it must go through a trusted project (in our case, via the tunnel).

See [Service Perimeters](https://cloud.google.com/vpc-service-controls/docs/create-service-perimeters)

First enable a security perimeter such that GCS access is only available via `project=gcp-project`:

![images/service_perimeter.png](images/service_perimeter.png)

> Note: once you enable this, all acess to GCS is blocked except via this specific project.


on a host 'on prem' force traffic for `www.googleapis.com` to _not_ go through the tunnel.  We can do this buy just making the DNS resolve to the external address.  On `instance-2`, remove the name resolution entry.

```
root@instance-2:~# more /etc/resolv.conf 
domain c.your-vpn.internal
search c.your-vpn.internal. google.internal.
#nameserver 192.168.0.2
nameserver 169.254.169.254
```

Then attempt to access GCS:  

(note the address is outside of the tunnel route: `74.125.70.95`)

```
root@instance-2:~# curl -vvvv -H "Authorization: Bearer ya29." https://www.googleapis.com/storage/v1/b?project=gcp-project-200601
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

```
root@instance-2:~# more /etc/resolv.conf 
domain c.your-vpn.internal
search c.your-vpn.internal. google.internal.
nameserver 192.168.0.2
nameserver 169.254.169.254
```

```
root@instance-2:~# curl -vvvv -H "Authorization: Bearer ya29.<redacted>" https://www.googleapis.com/storage/v1/b?project=gcp-project-200601
*   Trying 199.36.153.4...

< HTTP/2 200
{
 "kind": "storage#buckets",
```

# Conclusion

You've now got a tunnel from your onprem network through to google VMs you own...and as an added bonus, a private API endpoint for your use that will only be available via proxied projects you own.

To cleanup, just delete the projects and any service controls you setup for the APIs.

---

# References

- [Configuring Private Google Access for on-premises hosts](https://cloud.google.com/vpc/docs/configure-private-google-access-hybrid)
- [Using APIs from an External Network](https://cloud.google.com/solutions/using-gcp-apis-from-an-external-network)
- [Configuring Private Google Access](https://cloud.google.com/vpc/docs/configure-private-google-access)
