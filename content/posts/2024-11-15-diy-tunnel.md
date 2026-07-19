---
title: DIY Tunnel
date: 2024-11-15
---

# DIY Tunnel

I like to run my self-hosted services on machines in my home lab where possible. This reduces my cloud costs, especially for memory/storage-heavy applications. It also lets me keep my data "in-house", so to speak. My self-hosted services are often only ever exposed on my Tailscale Tailnet (meaning, they are never exposed directly to the public Internet). Still, some services need to be publicly exposed.

One way to do this is to punch holes in your firewall and point traffic at your ISP-provided IP. I'm always hesitant to do this. It exposes my home IP to any ne'er-do-well on the Internet, and I only have one public-facing IP to use.

A great option for HTTP-based services is [Cloudflare Tunnels](https://www.cloudflare.com/products/tunnel/?ref=straybits.ca). I am a big [fan of Cloudflare Tunnels](https://www.straybits.ca/2024/ghost-cloudflare-setup/). They let you run services on your LAN or home lab without exposing them directly to the Internet while benefiting from some of the protections Cloudflare offers. They also play well with Docker containers. Of course, doing so does put Cloudflare in the middle of your traffic. They handle TLS termination, and if they were evil, they'd be able to man-in-the-middle your traffic (I don't think they are evil).

The big problem with Cloudflare Tunnels is that they only work well with HTTP-based servers. If you are trying to expose a public-facing SMTP, IMAP, SSH, Minecraft, or... server, you are out of luck!

So, how can we set up our own less restrictive thing, like Cloudflare Tunnels, using a cheap VPS? How can we ensure our services see the correct IP for our inbound traffic? This post explains how.

> **Note:** Sample code for this tutorial can be found in this git repo.

# The Concept

The plan here is something like the following:

![](/media/diy-tunnel/image-39.png)

1. We have two machines:
   1. "**demo_vps**" (IP: 137.184.61.70) is our cheap cloud-based VPS with a clean Ubuntu install. Because this machine will only be routing traffic, we can mostly ignore CPU, RAM, and storage specs. Bandwidth is the only thing that matters.
   2. "**demo_private**" (behind our firewall) is our beefy home-lab server running the services we'd like to expose to the Internet. Let's assume this is also starting as a clean Ubuntu installation.
2. "demo_private" will connect to "demo_vps" using Wireguard to create an encrypted tunnel between the hosts.
3. Public traffic will be directed at the "demo_vps" and routed through the tunnel to "demo_private" so that the source IP for that traffic makes it through to "demo_private".

> **Note:** This example is entirely focused on IPv4. If you need to support IPv6, it should be possible with this approach, but you'll need many changes to the Wireguard and firewall configuration to support it.

# Routing?

The key piece that makes it difficult is the requirement that the underlying services *see* traffic from the correct source IP. If we aren't careful, it's easy to have our services see traffic appear to be coming from "demo_vps" which makes logs mostly useless but, more concerning, also creates a big problem with automated banning processes like fail2ban (banning all traffic from "demo_vps" instead of the malicious source IP – essentially blocking all traffic – speaking from experience).

![Things are bad when all incoming traffic looks the same](/media/diy-tunnel/dall-e-2024-11-15-07-53-04-a-photorealistic-mon.webp)

Some protocols, HTTP/HTTPS being one of them, support sneaking additional headers into the traffic to describe where traffic is coming from. Most HTTP proxy servers will do this, and many web applications can be configured to observe this rather than the connecting source IP. However... this doesn't work for many of the services I'd like to run (mail services and SSH, for example, which can't just have arbitrary headers stuffed into their messages).

Fortunately, with some not-so-complicated firewall magic, we can forward those packets right through our Wireguard tunnel so that they hit our "demo_private" machine with the correct source IP intact. When "demo_private" replies, those replies will go back through that tunnel, ensuring that the source IP the visitors see is that of "demo_vps".

As a bonus, this also means we aren't doing TLS termination on the VPS machine, which means it cannot intercept unencrypted traffic (at least not without minting a new SSL certificate). Encrypted traffic is being tunneled to "demo_private" which handles all the TLS magic. Nothing of value is stored on the VPS machine besides its own Wireguard private key!

# Basic Tunnel

Let's start with a simple setup and expose a local web service running on "demo_private".

On "demo_private", let's start up a basic webserver (if you don't have `python3` installed, you may have to install it). Do this in another terminal/SSH connection so that we can leave it running for testing.

> **Note:** This post assumes you don't already have a web server listening on port 80 on "demo_private". If you are following along, and you do, you can skip running this Python thing and just watch the logs of your actual web server.
```
mkdir test
cd test
echo "Hello, World" > index.html
python3 -m http.server 80
```
You'll see something like this when it's running:
```
root@demo-private:~/test# python3 -m http.server 80
Serving HTTP on 0.0.0.0 port 80 (http://0.0.0.0:80/) ...
```
And, if you visit `http://[demo-private-ip]` in your browser, you'll see log messages like this (not the source IP is part of the log messages):
```
192.168.1.103 - - [13/Nov/2024 15:14:27] "GET / HTTP/1.1" 304 -
```
Great. Leave that open in a terminal, and let's set up our first Wireguard tunnel and expose this sucker to the Internet!

First, we'll install Wireguard on each machine:
```
sudo apt install wireguard
```
Wireguard uses public/private keypairs to authenticate between nodes, so we'll need to generate a key pair on both machines using the following command:
```
wg genkey | tee privatekey | wg pubkey > publickey
```
The generated private key will be written to `privatekey` and should stay on the machine it was generated on. The generated public key is written to `publickey` and will be copied into the Wireguard configuration on the other side.

So, on "demo-vps": (your keys will be different from mine and yes, obviously, these keys were throw-away)
```
root@demo-vps:~# wg genkey | tee privatekey | wg pubkey > publickey
root@demo-vps:~# cat privatekey
uH0mXITFCZ2jObGS3+ZMKe+SvvYb89lbUjWbsAZP1UA=
root@demo-vps:~# cat publickey
5vtJq36XyI53MWx6nf/bhmxVpbmNr3mtB2HoqAk9MR0=
```
And on "demo-private":
```
root@demo-private:~# wg genkey | tee privatekey | wg pubkey > publickey
root@demo-private:~# cat privatekey
IFYA34ZKiPKtae36sc8NE87NK/Z34yNklweNf3374Hs=
root@demo-private:~# cat publickey
c1Aclgb/JkyRKGYAJzdq82mbtrNP+IHayUUI73Le4Rw=
```
Now we'll configure Wireguard on "demo-vps" by creating `/etc/wireguard/wg0.conf`.
```
[Interface]
Address = 10.0.0.1/24         # Private IP for the VPS in the VPN network
ListenPort = 51820            # Default WireGuard port
# Private key is the Private key from "demo-vps"
PrivateKey = uH0mXITFCZ2jObGS3+ZMKe+SvvYb89lbUjWbsAZP1UA=

# packet forwarding
PreUp = sysctl -w net.ipv4.ip_forward=1

# port forwarding (HTTP, HTTPS) - update port list as required 
PreUp = iptables -t nat -A PREROUTING -i eth0 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 10.0.0.2
PostDown = iptables -t nat -D PREROUTING -i eth0 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 10.0.0.2

[Peer
# PublicKey is the public key for "demo_private"
PublicKey = c1Aclgb/JkyRKGYAJzdq82mbtrNP+IHayUUI73Le4Rw=
AllowedIPs = 10.0.0.2/32      # IP of the private server in VPN
```
This is fairly straightforward:

1. We enable IP forwarding when this tunnel comes up to allow the kernel to forward packets across interfaces.
2. We add some routing rules that take incoming traffic to port 80 or 443 and forward it to the Wireguard IP of "demo_private".
   1. You can easily add additional TCP services/ports by updating both occurrences of "80,443" to include the additional TCP ports. If you need UDP ports, you must add more rules.

> **Note:** Be careful when forwarding additional ports that you don't inadvertently lock your self out of the VPS system. For example, don't forward port 22 (SSH) to "demo_private" if that's how you are connecting to "demo_vps"!

You can *temporarily* start up Wireguard with:
```
sudo wg-quick up wg0
```
Now, let's configure Wireguard on "demo-private" by creating `/etc/wireguard/wg0.conf` on our "demo-private" machine.
```
[Interface]
Address = 10.0.0.2/24         # Private IP for the private server in the VPN network
# Enter the private key of our "demo-private" machine
PrivateKey = IFYA34ZKiPKtae36sc8NE87NK/Z34yNklweNf3374Hs=
Table = 123

# Enable IP forwarding
PreUp = sysctl -w net.ipv4.ip_forward=1

# Return traffic through wireguard
PreUp = ip rule add from 10.0.0.2 table 123 priority 1
PostDown = ip rule del from 10.0.0.2 table 123 priority 1

[Peer]
# Enter the public key for "demo-vps"
PublicKey = 5vtJq36XyI53MWx6nf/bhmxVpbmNr3mtB2HoqAk9MR0=
# Because we are passing packets from the VPS, the source IP will
# be our client IPs (i.e. we can't just fix this to 10.0.0.1)
AllowedIPs = 0.0.0.0/0
# Adjust to the actual IP of "demo-vps"
Endpoint = 137.184.61.70:51820
PersistentKeepalive = 25
```
This one is a bit more complicated than on "demo_vps":

1. Here, we're defining a custom routing table for our Wireguard tunnel `Table = 123`.
2. We add a firewall rule to tie all traffic from 10.0.0.2 (the Wireguard IP on "demo_private") to that routing table (meaning that traffic will go through the Wireguard tunnel and look like it's originating from "demo_vps"). This means return traffic will be passed through that tunnel, which is what we want.
3. If you are wondering about the `AllowedIPs = 0.0.0.0/0` line, we need that to let Wireguard know that we'll accept traffic from any address from "demo_vps" (required since we're forwarding packets with source IP intact; if we locked it down to 10.0.0.1/32, we'd block all traffic except that originating from the VPS itself).

At this point, after running `sudo wg-quick up wg0` on "demo-private", we should have a working tunnel.

Running `wg` on either machine should show an established tunnel, and if I hit my VPS IP in the browser `http://137.184.61.70`, I'll see my "Hello, World" page.

![](/media/diy-tunnel/image-42.png)

You'll also notice that my web server logs my visiting public IP address, not an internal one like 10.0.0.1.

![](/media/diy-tunnel/image-40.png)

At this point, if everything is working correctly, you may want to configure your Wireguard tunnel to survive a reboot by running this on each machine:
```
sudo systemctl enable wg-quick@wg0
```
Success!

# But... Docker?!

Of course, you may have noticed in previous posts that I ❤️ running all my services in Docker this makes things a bit more complicated because of how Docker handles networking. There are a couple of ways we can attack this.

1. Running Wireguard on the Host (as we did above) and making more complex firewall rules to handle routing traffic to our Docker containers
2. Moving Wireguard into Docker

## Wireguard on Host

![](/media/diy-tunnel/image-43.png)

This creates a complication because, even though you might expose a Docker service on port 80 of your host machine, it's not really listening on your host's IP addresses. Some firewall rules forward traffic from your local machine to the internal Docker container IP. These rules don't play well with the rules we created for our Wireguard tunnel.

For example, if I stop the Python-based web server and run a simple web server in Docker instead, you'll notice we don't see responses from our server.
```
docker run -it --rm -p 80:80 --name testserver traefik/whoami
```
From another host...
```
$ curl http://137.184.61.70/
...
...
BUELLER
BUELLER
```
Fortunately, we can adjust our rules a bit in our private machine's `wg0.conf`. We'll replace the fairly simple:
```
# Return traffic through wireguard
PreUp = ip rule add from 10.0.0.2 table 123 priority 1
PostDown = ip rule del from 10.0.0.2 table 123 priority 1
```
With something a bit more complex. The concept here is that as we see packets coming in from the `wg0` interface, we're "marking" them. We also apply that same mark to *return packets* for those marked incoming packets. Finally, we're saying that all marked packets are routed (using our "123" routing table) through the Wireguard tunnel.
```
# loose reverse path forwarding validation
PostUp = sysctl -w net.ipv4.conf.wg0.rp_filter=2

# Mark new connections coming in through wg0
PreUp = iptables -t mangle -A PREROUTING -i wg0 -m state --state NEW -j CONNMARK --set-mark 1
PostDown = iptables -t mangle -D PREROUTING -i wg0 -m state --state NEW -j CONNMARK --set-mark 1

# Mark return packets to go out through WireGuard via policy routing
PreUp = iptables -t mangle -A PREROUTING ! -i wg0 -m connmark --mark 1 -j MARK --set-mark 1
PostDown = iptables -t mangle -D PREROUTING ! -i wg0 -m connmark --mark 1 -j MARK --set-mark 1

# Push marked connections back through wg0
PreUp = ip rule add fwmark 1 table 123 priority 456
PostDown = ip rule del fwmark 1 table 123 priority 456
```
Making these changes and restarting Wireguard `wg-quick down wg0; wg-quick up wg0` will get things up and running again.
```
$ curl http://137.184.61.70/
Hostname: 8d092c421d2a
IP: 127.0.0.1
IP: ::1
IP: 172.17.0.2
RemoteAddr: 184.64.122.150:39524
GET / HTTP/1.1
Host: 137.184.61.70
User-Agent: curl/8.5.0
Accept: */*
```
Notice that "RemoteAddr" is my actual source IP

Awesome!

However, another approach, and my preferred approach, is to avoid running Wireguard on the host machine and keep everything within our Docker network.

## Wireguard in Docker Compose

![](/media/diy-tunnel/image-44.png)

Another option that works well if all the services you are exposing happen to be defined in the same `docker-compose.yml` file is to set up a Wireguard container in that `docker-compose.yml` file and update the other services to *bind their network to that of the Wireguard container.*

So, disable Wireguard on the host machine ("demo_private").

Let's create `/docker/test/docker-compose.yml` as follows:
```docker
services:

  wireguard:
    image: lscr.io/linuxserver/wireguard:latest
    hostname: demo_private
    cap_add:
      - NET_ADMIN
    environment:
      - TZ=America/Edmonton
    volumes:
      - ./wg0.conf:/config/wg_confs/wg0.conf
    restart: always
    sysctls:
      - net.ipv4.ip_forward=1

  server:
    image: traefik/whoami
    restart: always
    # this is the special sauce.  This attaches this container to the
    # network context of the wireguard container.  
    network_mode: service:wireguard
```
The important parts from above:

1. We've created a new container for Wireguard and bind-mounted `wg0.conf` (from the same folder) into our container.
2. We are also enabling IP forwarding here rather than in `wg0.config`.
3. The most important part is that we are tying the network context of our web server to the Wireguard container. This means that our web server is essentially listening on port 80 of that Wireguard container, so when we're forwarding traffic to 10.0.0.2:80, it #JustWorks. If we had multiple services (say SMTP, SMTPS, IMAP, ...) spread across multiple containers in the same `docker-compose.yml` file, we could add that same `network_mode: service:wireguard` to each of those containers, too (as long as those services don't have overlapping ports!).
4. Notice that we also don't have exposed ports from our "server" container above. We don't need to expose ports to the host machine, and Wireguard doesn't need them exposed because they share the same network context.

An our `/docker/test/wg0.conf` file:
```
[Interface]
Address = 10.0.0.2/24         # Private IP for the private server in the VPN network
# Enter the private key of our "demo-private" machine (NOW IN DOCKER)
PrivateKey = IFYA34ZKiPKtae36sc8NE87NK/Z34yNklweNf3374Hs=
Table = 123

# Routing
PreUp = ip rule add from 10.0.0.2 table 123 priority 1
PostDown = ip rule del from 10.0.0.2 table 123 priority 1

[Peer]
# Enter the public key for "demo-vps"
PublicKey = 5vtJq36XyI53MWx6nf/bhmxVpbmNr3mtB2HoqAk9MR0=
AllowedIPs = 0.0.0.0/0
# Adjust to the actual IP of "demo-vps"
Endpoint = 137.184.61.70:51820
PersistentKeepalive = 25
```
Notice that the above looks very similar to our basic case when we started this adventure, except that we've removed the PreUp rule that was enabling IP forwarding and moved that into `docker-compose.yml` for permission-related reasons.

Starting this up with `docker compose up` and running our test again shows that we have this working with the correct source IPs hitting our underlying service running on "demo_private" (this time, entirely in Docker).
```
$ curl http://137.184.61.70/
Hostname: demo_private
IP: 127.0.0.1
IP: ::1
IP: 10.0.0.2
IP: 172.18.0.2
RemoteAddr: 184.64.122.150:38200
GET / HTTP/1.1
Host: 137.184.61.70
User-Agent: curl/8.5.0
Accept: */*
```
This example gets me the closest to what I love about Cloudflare tunnels. Assuming all my containers are storing their data in the local directory (my strong preference for vs. using Docker volumes) this means I can move this folder (containing `docker-compose.yml`, `wg0.conf` and all my container data) to any host running docker, start it up, and they'll connect to my VPS, and everything will work!

# Bonus

## Multiple Tunnels

The example above shows a single VPS "demo_vps" and a single private server "demo_private". If you have multiple internal servers that hold services you'd like to expose via. a single VPS, you can easily establish additional tunnels.

You'd end up generating additional keypairs for the additional private servers and then update the "demo_vps" `wg0.conf` file something like the following:
```
[Interface]
Address = 10.0.0.1/24         # Private IP for the VPS in the VPN network
ListenPort = 51820            # Default WireGuard port
# Private key is the Private key from "demo-vps"
PrivateKey = uH0mXITFCZ2jObGS3+ZMKe+SvvYb89lbUjWbsAZP1UA=

# packet forwarding
PreUp = sysctl -w net.ipv4.ip_forward=1

# port forwarding (HTTP, HTTPS) to private 1 
PreUp = iptables -t nat -A PREROUTING -i eth0 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 10.0.0.2
PostDown = iptables -t nat -D PREROUTING -i eth0 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 10.0.0.2

# port forwarding (SSH) to private 2 
PreUp = iptables -t nat -A PREROUTING -i eth0 -p tcp -m multiport --dports 22 -j DNAT --to-destination 10.0.0.3
PostDown = iptables -t nat -D PREROUTING -i eth0 -p tcp -m multiport --dports 22 -j DNAT --to-destination 10.0.0.3

# port forwarding (SMTP) to private 3 
PreUp = iptables -t nat -A PREROUTING -i eth0 -p tcp -m multiport --dports 25 -j DNAT --to-destination 10.0.0.4
PostDown = iptables -t nat -D PREROUTING -i eth0 -p tcp -m multiport --dports 25 -j DNAT --to-destination 10.0.0.4

[Peer]
# PublicKey is the public key for "demo_private_1"
PublicKey = c1Aclgb/JkyRKGYAJzdq82mbtrNP+IHayUUI73Le4Rw=
AllowedIPs = 10.0.0.2/32      # IP of the private server in VPN

[Peer]
# PublicKey is the public key for "demo_private_2"
PublicKey = Another Key from Private 2
AllowedIPs = 10.0.0.3/32      # IP of the private server in VPN

[Peer]
# PublicKey is the public key for "demo_private_3"
PublicKey = Another key from private_3
AllowedIPs = 10.0.0.4/32      # IP of the private server in VPN
```
The configuration of the internal/private services would remain the same. Now, from the VPS, after restarting Wireguard and bringing up the tunnels, when you run `wg` you'll see multiple established tunnels.

> **Note:** If the goal is to expose multiple HTTP/HTTPS-based services on the standard ports, you'll need to point 80/443 at a proxy like Caddy or NgingProxyManager and have that dish out traffic to the correct internal services.

## Hairpinning

Another minor annoyance is that if services on "demo_private" are trying to call other services on "demo_private" by their public name (i.e. I have a web app and mail server running on "demo_private" and the web app tries to initiate a mail message), that traffic will go out to the Internet and back in through the VPS. While it works, this seems wasteful. We can fix this by adding another firewall rule (below the existing PreUp/PostDown lines) that will rewrite traffic from "demo_private" to our public IP to point at localhost instead.
```
# Route traffic to public IP to self to avoid it hitting the network
PreUp = iptables -t nat -A OUTPUT -d 137.184.61.70 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 127.0.0.1
PostDown = iptables -t nat -D OUTPUT -d 137.184.61.70 -p tcp -m multiport --dports 80,443 -j DNAT --to-destination 127.0.0.1
```
## Inbound Firewall Rules

As configured above, the VPS calls the shots as to what traffic is being forwarded to "demo_private". This means a compromised VPS machine could use our established tunnel to poke at services we never intended to expose. We can add some additional firewall rules in `wg0.conf` on our "demo_private" side to clamp this down.
```
# Allow our expected traffic
PreUp = iptables -A INPUT -i wg0 -p tcp -m multiport --dports 80,443 -j ACCEPT
PostDown = iptables -D INPUT -i wg0 -p tcp -m multiport --dports 80,443 -j ACCEPT

# And pings
PreUp = iptables -A INPUT -i wg0 -p icmp --icmp-type echo-request -j ACCEPT
PostDown = iptables -D INPUT -i wg0 -p icmp --icmp-type echo-request -j ACCEPT

# Block the rest
PreUp = iptables -A INPUT -i wg0 -j DROP
PostDown = iptables -D INPUT -i wg0 -j DROP
```
These would go near the bottom, just under the other PreUp/PostDown lines

The above would ensure that the private server only permits HTTP, HTTPS, and ping traffic from the `wg0` interface.

---

*Originally published at [onewheelgeek.me](https://onewheelgeek.me/posts/diy-tunnel/).*
