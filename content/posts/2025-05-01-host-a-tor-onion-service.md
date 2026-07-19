---
title: Host a Tor Onion Service
date: 2025-05-01
---

# Host a Tor Onion Service

Ever wanted to host a website that can’t easily be traced back to you? [Tor](https://www.torproject.org/?ref=straybits.ca), the anonymizing network, supports a feature called an [o*nion service*](https://tb-manual.torproject.org/onion-services/?ref=straybits.ca) — essentially, a service (like a web server) that’s only accessible from within the Tor network. The identities/location of both the server operator *and* the visitors are reasonably well protected.

It's a fun little project, and in today's political climate, who knows — it might come in handy someday.

> **Note:** This guide is not meant for people who need high-security anonymity (like whistleblowers, journalists, or activists in hostile environments). This setup is pretty good for hobbyists or developers who want to understand how Tor hidden services work, but if your freedom or safety is on the line, you’ll need a much more hardened setup.

## Overview

We’re going to deploy a hidden web server using Docker. Our setup includes two containers:

- `web` – a lightweight [NGINX](https://nginx.org/?ref=straybits.ca) server to host a static website
- `tor` – a container running the Tor daemon, configured to expose a hidden service

> **Note:** Dynamic languages (PHP, Node, Python) open extra attack surfaces—databases, interpreters, session cookies, user input, you name it. A pure-static site, such as that deployed in this post, has zero server-side code, so an attacker can’t exploit what isn’t there.

These containers share a private Docker network, so the *only* way to reach the web server is through the Tor network.

View this [entire project here](https://git.straybits.ca/straybits/docker-samples/src/branch/main/onion-service?ref=straybits.ca).

## Why NGINX?

NGINX has a few key advantages:

- Minimal resource usage
- Excellent security record
- No outgoing connections or built-in telemetry

This means we’re less likely to leak metadata or create unwanted traffic patterns that could compromise privacy.

## Folder Structure

We’ll create a directory (e.g. `/docker/mysite`) with the following structure:
```
mysite/
├── data/                 # Runtime state for Tor
├── docker-compose.yml    # Docker Compose config
├── nginx.conf            # NGINX config
├── site/                 # Your website content
│   └── index.html
└── tor/                  # Tor container config
    ├── Dockerfile
    └── torrc
```
## Tor Dockerfile (`tor/Dockerfile`)

We’ll build our own Docker image instead of using a random one from Docker Hub. This ensures we use a current, official build of Tor in a minimal container:
```dockerfile
FROM debian:12-slim

RUN apt-get update && apt-get install -y --no-install-recommends gnupg wget ca-certificates

RUN wget -qO- https://deb.torproject.org/torproject.org/A3C4F0F979CAA22CDBA8F512EE8CBC9E886DDD89.asc \
     | gpg --dearmor -o /usr/share/keyrings/tor-archive-keyring.gpg

RUN echo \
  "deb [signed-by=/usr/share/keyrings/tor-archive-keyring.gpg] \
  https://deb.torproject.org/torproject.org bookworm main" \
  > /etc/apt/sources.list.d/tor.list

RUN apt-get update \
 && apt-get install -y --no-install-recommends tor \
 && rm -rf /var/lib/apt/lists/*

USER debian-tor                      
COPY torrc /etc/tor/torrc
VOLUME ["/var/lib/tor"]
CMD ["tor", "-f", "/etc/tor/torrc"]
```
## Tor Config (`tor/torrc`)

This config sets up the hidden service directory and maps port 80 on the hidden service to our internal web server:
```
Log notice stdout

HiddenServiceDir /var/lib/tor/hs_site
HiddenServiceVersion 3
HiddenServicePort 80 web:80

SocksPort 0
ClientOnly 1
ExitRelay 0
```
## Docker Compose (`docker-compose.yml`)

Here’s the full `docker-compose.yml` file to wire it all together:
```yaml
services:
  web:
    image: nginx:alpine
    read_only: true
    volumes:
      - ./site:/usr/share/nginx/html:ro
      - ./nginx.conf:/etc/nginx/nginx.conf:ro   
    tmpfs:
      - /var/cache/nginx
      - /var/cache/nginx/client_temp
      - /var/cache/nginx/proxy_temp
      - /var/cache/nginx/fastcgi_temp
      - /var/cache/nginx/uwsgi_temp
      - /var/cache/nginx/scgi_temp
      - /tmp    
    user: ${NGINX_UID}:${NGINX_GID}
    security_opt: [ no-new-privileges:true ]
    networks: [ hidden ]
    restart: unless-stopped

  tor:
    build: ./tor
    volumes:
      - ./data/tor:/var/lib/tor
    read_only: true
    cap_drop: [ ALL ]
    security_opt: [ no-new-privileges:true ]
    networks: [ hidden, tor_out ]
    depends_on: [ web ]
    healthcheck:
      test: ["CMD-SHELL", "tor --verify-config -f /etc/tor/torrc"]
      interval: 30s
      timeout: 10s
      retries: 3
    restart: unless-stopped

networks:
  hidden:
    internal: true              # private LAN – no Internet
  tor_out:
    driver: bridge              # only Tor joins – outbound TCP allowed
```
## Environment Variables (`.env`)

Create a `.env` file with the UID/GID NGINX will run as:
```
NGINX_UID=101
NGINX_GID=101
```
## Sample Web Page

Create a basic site and adjust permissions so that only our NGINX process can access it (making it harder for other non-root processes on the same box from discovering the content):
```bash
mkdir -p site
echo "Hello World!" > site/index.html
. .env && chown -R $NGINX_UID:$NGINX_GID site
```
## Initialize Tor Data Directory

Tor (under Debian) runs as user `100` by default, so we need to create a directory for Tor to store its state that can be R/W by that user.
```bash
mkdir -p data/tor
chown 100:100 data/tor
```
## Run It
```bash
docker compose up -d
```
Once it starts, grab your onion address:
```bash
cat data/tor/hs_site/hostname
```
You should see something like:
```
zwh3yrsq7gqpgx6qj6z3or36dojxj4mvlaefoqg5vljtrlruqk5sz2qd.onion
```
Visit it using the [Tor browser](https://www.torproject.org/download/?ref=straybits.ca)!

![](/media/host-a-tor-onion-service/image-1.png)

And clicking the path icon shows our connectivity to our site bouncing through the Tor network.

![Showing the Tor 'circuit' used to access an onion service](/media/host-a-tor-onion-service/image-2.png)

## Updating Content Remotely

So your onion site is now live in its secret bunker, but how do you update the files without blowing your cover? A direct SSH/SFTP connection from your home IP would leave footprints, and poking inbound holes in firewalls or VPNs defeats the point of “hidden.”

**Solution:** spin up a *second* onion service dedicated to SFTP. By giving it its own `.onion` address (and keeping that URL to yourself), you get a Tor-only file‐transfer tunnel that never touches the public internet—and visitors to the web address won’t even know the SFTP endpoint exists.

*Why a separate service?*

- **Compartmentalization:** If attackers discover your public site’s onion URL, they still have no path to your SFTP host.
- **Access control:** You can lock the SFTP service with strong Tor authentication (v3 client keys) or one-off SSH keys without affecting the web front-end.

In short, treat content management as its own walled-off workflow: same anonymity guarantees and zero IP exposure.

First, let's add a new component to our `docker-compose.yml` file for SFTPGo and add it as a dependency to our tor container.
```docker
  sftp:
    image: drakkan/sftpgo:latest             
    environment:
      - SFTPGO_HTTPD__ENABLED=true
      - SFTPGO_FTPD__ENABLED=false
      - SFTPGO_WEBDAVD__ENABLED=false

      - SFTPGO_DATA_PROVIDER__CREATE_DEFAULT_ADMIN=true
      - SFTPGO_DEFAULT_ADMIN_USERNAME=admin
      - SFTPGO_DEFAULT_ADMIN_PASSWORD=${ADMIN_PASSWORD}

      - SFTPGO_AUTH__PASSWORD_LOGIN_DISABLED=true
    volumes:
      - ./site:/data/site:rw                 
      - ./data/sftp:/var/lib/sftpgo          
    user: ${SFTP_UID}:${SFTP_UID}
    security_opt: [ no-new-privileges:true ]
    networks: [ hidden ]
    restart: unless-stopped

  tor:
    # .....
    depends_on: [ web, sftp ]
```
Next, let's add some new variables to our `.env` file:
```env
NGINX_UID=101
NGINX_GID=101

# can finess permissions further to ensure NGINX can read 
# content but not write it.
SFTP_UID=101
SFTP_GID=101

# Admin account for setting up SFTP users on SFTPGo
ADMIN_PASSWORD=changeme
```
And finally, update `tor/torrc` and add another onion service for SFTPGo (exposing SSH and HTTP).
```
Log notice stdout

HiddenServiceDir /var/lib/tor/hs_site
HiddenServiceVersion 3
HiddenServicePort 80 web:80

HiddenServiceDir /var/lib/tor/hs_sftp
HiddenServiceVersion 3
HiddenServicePort 22 sftp:2022 
HiddenServicePort 80 sftp:8080 

SocksPort 0
ClientOnly 1
ExitRelay 0
```
Finally, let's create the `data/sftp` folder with appropriate permissions.
```bash
mkdir -p data/sftp
. .env && chown $SFTP_UID:$SFTP_UID data/sftp
```
Rebuild your docker image with `docker compose build` and start it back up with `docker compose up -d`.

From there, we can get the hidden service location:
```bash
cat data/tor/hs_sftp/hostname
```
And when you hit that URL in your Tor Browser, you should see the login page for SFTPGo:

![](/media/host-a-tor-onion-service/image-3.png)

Now click "Go to WebAdmin" and login with "admin" and the password you put in your `.env` file and from there you can create a new user.

- If anonymity is key, and why wouldn't it be, make sure to use a generic username.
- Don't specify a password. Password-based login is disabled anyways.
- Instead, add an SSH key (make sure it's a fresh one used just for this purpose). It defeats the purposes if you are embedding a cryptographically strong identifier publicly known to identify you into the configuration of your hidden service. (i.e. don't use the same SSH key you use for Github)
```
❯ ssh-keygen -t ed25519 -f secret
Generating public/private ed25519 key pair.
Enter passphrase for "secret" (empty for no passphrase):
Enter same passphrase again:
Your identification has been saved in secret
Your public key has been saved in secret.pub
The key fingerprint is:
SHA256:A9MM4NVmgmXL91N5m9yRZiqXXj42a8MdH3qX6BmycIE jsc@watts.local
The key's randomart image is:
+--[ED25519 256]--+
|    .+=.         |
|   ..+.=+    .  .|
|    . =++   o .= |
|       + ... o=+.|
|        SEoo ++..|
|         . .= oo |
|         . o o+=*|
|          o oo+*B|
|           ..oo.o|
+----[SHA256]-----+
❯ cat secret.pub
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMaXxhxFilNJxWGhthqZA+/Udyikijw3ZZg6f6aA6pTe jsc@watts.local
```
![](/media/host-a-tor-onion-service/image-4.png)

And make sure to point it at /data/site so that the SFTP user can only see our page content.

![](/media/host-a-tor-onion-service/image-5.png)

Now we should be able to connect to it using `torify` or the Tor socks proxy.
```bash
torify sftp -i secret content@3evjjf7birgzsyqwt3hekhb3o2tyyjoz4wacbu2g2qvcoi46xl3ioqid.onion
```
```
❯ torify sftp -i secret content@3evjjf7birgzsyqwt3hekhb3o2tyyjoz4wacbu2g2qvcoi46xl3ioqid.onion
The authenticity of host '3evjjf7birgzsyqwt3hekhb3o2tyyjoz4wacbu2g2qvcoi46xl3ioqid.onion (127.42.42.0)' can't be established.
ED25519 key fingerprint is SHA256:SFl9klK+GV6H/EI/V2veK0rL1kzbhBJpblbHDj6gnfI.
This key is not known by any other names.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '3evjjf7birgzsyqwt3hekhb3o2tyyjoz4wacbu2g2qvcoi46xl3ioqid.onion' (ED25519) to the list of known hosts.
Connected to 3evjjf7birgzsyqwt3hekhb3o2tyyjoz4wacbu2g2qvcoi46xl3ioqid.onion.
sftp> ls
index.html
sftp> put hello.txt
Uploading hello.txt to /hello.txt
hello.txt                                                       100%    3     0.0KB/s   00:00
sftp>
```
Oooohhhhh...

At this point, if you aren't managing additional SSH user, it would probably be a good idea to update your `torrc` file and remove the SFTP HTTP to reduce attack surface.

## A Note on Deanonymization Risks

Just because your service is on Tor doesn’t mean you’re bulletproof.

### Shared Infrastructure

Running your hidden service on the same hardware or network as other services (e.g., your personal website) can expose you to correlation attacks. For example:

> “Hey, both jeff.com *and* this .onion site go offline at the exact same time... hmm.”

Mitigation: isolate services and use dedicated infrastructure.

### Static Content Can Still Give You Away

Static ≠ invisible. Even a simple HTML page can leak clues about who you are or where you are located:

| Hidden Leak | What to Watch For |
| --- | --- |
| **EXIF data in images** | Strip GPS and camera metadata. |
| **Unique fonts / CSS fingerprints** | Re-use of rare web fonts or identical CSS hashes can link your onion site to a clearnet one. Stick to system fonts or popular open-source font kits you self-host. |
| **External resources** | Pull everything local. Third-party JS/CSS (Google Fonts, Hotjar, CDNs) forces Tor exit nodes to fetch clearnet assets that could log requests or set tracking cookies. |
| **Build artifacts** | Some generators inject timestamps or absolute build paths into HTML comments. Disable those options or scrub the output. |

End result: a static site is easier to secure—but only if you audit the assets you publish.

### Docker Security

Docker is great for isolating processes and managing deployments, but it’s not a silver bullet for security — especially when you’re relying on it to keep a service *truly hidden*.

In this configuration, we've taken steps to:

- Run containers as non-root users
- Run containers with read-only filesystem/tmpfs

But Docker doesn't guarantee kernel-level isolation. If there is a vulnerability in the kernel, a compromised container could potentially break out and affect the host, or leak information about it's actual location/network.

Using virtual machines (VMs) provides stronger isolation than containers alone. You can also reduce the risk of information leakage by enforcing strict network and firewall rules — ensuring that only the Tor container can communicate with the web container and nothing else can reach it.

## Upgrades

It's always a good idea to stay current. You can update everything with...
```bash
# pull latest images for nginx and SFTPGo containers
docker compose pull

# rebuild our custom Tor image
docker compose build --no-cache

# restart all the services on the latest & greatest
docker compose up -d
```
> **Note:** Operating an onion service is legal in most jurisdictions, but what you host may not be. Know your local laws.

---

*Originally published at [onewheelgeek.me](https://onewheelgeek.me/posts/host-a-tor-onion-service/).*
