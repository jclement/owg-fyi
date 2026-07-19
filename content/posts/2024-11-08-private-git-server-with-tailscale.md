---
title: Private Git Server with Tailscale
date: 2024-11-08
---

# Private Git Server with Tailscale

In this post, I'll be setting up a full-featured private Git server (running [Forgejo](https://forgejo.org/?ref=straybits.ca)) on my [Tailscale](https://www.tailscale.com/?ref=straybits.ca) Tailnet. For more background and detail on the Forgejo side of this, see our [previous post](https://www.straybits.ca/2024/self-hosted-git-server/) on configuring a public instance of Forgejo using Cloudflare.

> **Note:** The full code for this tutorial can be found in this git repo.

Let's start simple, with a standalone Forgejo server (without build services) attached to my Tailnet.

First, let's generate a new TS Auth Key from the Tailscale UI (Settings > Keys).

![](/media/private-git-server-with-tailscale/image-35.png)

This will generate a auth key that looks like `tsauth-key-....`. Copy this, you'll need to put that into your `.env` file in a moment.

On a host machine with Docker installed, we'll create a `docker-compose.yml` file which will define our services, a `.env` file which will contain some configuration, and `ts-serve.json` which will configure Tailscale to share the git server on the Tailnet. We'll put all these files in a single directory, such as `/docker/privategit`.

Here is my starting `docker-compose.yml`. Notice that it defines containers for "tailscale", "server" (Forgejo) and "db" (Postgres). While it is certainly possible to install Tailscale on the host machine, and share various Docker containers from there, I prefer to connect each container individually to Tailscale so that each container has a nice name on my Tailnet like `git.XXX.ts.net` instead of `server.XXX.ts.net:1234`. Attaching each container individually to the Tailnet also makes them portable between hosts (i.e. I can move the whole folder to a new machine, `docker compose up` and it'll just work)!
```docker
services:

  tailscale:
    hostname: ${TAILNET_NAME}             
    image: tailscale/tailscale
    volumes:
      - ./data/tailscale:/var/lib/tailscale
      - ./ts-serve.json:/config/ts-serve.json:ro
      - /dev/net/tun:/dev/net/tun         
    cap_add:                          
      - net_admin
      - sys_module
    environment:
      TS_AUTHKEY: ${TS_AUTHKEY}
      TS_SERVE_CONFIG: /config/ts-serve.json
      TS_AUTH_ONCE: true
      TS_STATE_DIR: /var/lib/tailscale
      TS_HOST: ${TAILNET_NAME}
    restart: unless-stopped

  server:
    image: codeberg.org/forgejo/forgejo:${FORGEJO_TAG}
    environment:
      # https://forgejo.org/docs/latest/admin/config-cheat-sheet/
      - RUN_MODE=prod
      - USER_UID=1000
      - USER_GID=1000
      - FORGEJO__server__ROOT_URL=https://${FORGEJO_HOSTNAME}
      - FORGEJO__server__SSH_DOMAIN=${FORGEJO_HOSTNAME}
      - FORGEJO__database__DB_TYPE=postgres
      - FORGEJO__database__HOST=db:5432
      - FORGEJO__database__NAME=gitea
      - FORGEJO__database__USER=gitea
      - FORGEJO__database__PASSWD=${FORGEJO_DB_PASSWORD}
    restart: always
    volumes:
      - ./data/data:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    depends_on:
      - db

  db:
    image: postgres:13
    restart: always
    environment:
      - POSTGRES_USER=gitea
      - POSTGRES_PASSWORD=${FORGEJO_DB_PASSWORD}
      - POSTGRES_DB=gitea
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
```
> **Note:** I give Forgejo hints for FORGEJO__server__SSH_DOMAIN and FORGEJO__server__ROOT_URL because otherwise it guesses wrong and generates URLs with localhost in them.

The above `docker-compose.yml` file references a handful of variables, so we'll need to define those in `.env`. Make sure to set TS_AUTHKEY, your TAILNET_SUFFIX, and generate a FORGEJO_DB_PASWORD.
```env
FORGEJO_TAG=8
FORGEJO_RUNNER_TAG=4.0.1

# Tailscale authorization key
TS_AUTHKEY=tskey-auth-

# Tailscale tailnet node name
TAILNET_NAME=git
TAILNET_SUFFIX=XXXXXX.ts.net

# Instance Settings
FORGEJO_HOSTNAME=${TAILNET_NAME}.${TAILNET_SUFFIX}

# Database
FORGEJO_DB_PASSWORD= ##REQUIRED##
```
And finally, we're going to use [Tailscale Serve](https://tailscale.com/kb/1242/tailscale-serve?ref=straybits.ca) to publish our Git server on our Tailnet and handle getting a TLS certificate for it. This requires a `ts-serve.json` (which you'll notice is bind-mounted into our Tailscale container above).

- We are exposing two ports with Tailscale Serve here:
  - TCP/22 (for SSH access to the Git server)
  - HTTPS/443 (for HTTPS access to the Git server UI)
- Both are proxying traffic through to the `server` container (22, and 80, respectively).
- Tailscale Serve automatically handles getting a TLS certificate for our HTTPS service.
```
{
    "TCP": {
        "22": {
            "TCPForward": "server:22"
        },
        "443": {
            "HTTPS": true
        }
    },
    "Web": {
        "${TS_CERT_DOMAIN}:443": {
            "Handlers": {
                "/": {
                    "Proxy": "http://server:3000"
                }
            }
        }
    },
    "AllowFunnel": {
        "${TS_CERT_DOMAIN}:443": false
    }
}
```
At this point, you should be able to run `docker compose up` and it'll pull the container images, start everything up, and create the node on your Tailnet. After a couple of minutes, you should be able to hit `https://git.XXXXXX.ts.net` in your browser and see the Forgejo setup screen!

Behind the scenes, this is how things are wired up:

![](/media/private-git-server-with-tailscale/image-36.png)

The Git server is only available to other machines on your Tailnet. However, if you want, you can also change the "AllowFunnel" section to true and now, using [Tailscale Funnels](https://tailscale.com/kb/1223/funnel?ref=straybits.ca), the HTTPS side of your service will be publicly available by the same name. Tailscale Funnels don't support non-HTTP/HTTPS services.

> **Note:** I like using Tailscale Serve in this way because the ts-serve.config makes it easy to expose services from multiple containers as a single host. Previously, I used Caddy for this, but this is fewer moving parts.

If all you need is a basic Git server, you are probably good to go. The [full example in the Git Repo](https://git.straybits.ca/straybits/docker-samples/src/branch/main/forgejo_tailscale?ref=straybits.ca) performs most of the setup steps automatically and includes Forgejo Runner which lets you run Github Action-style actions upon pushes.

---

*Originally published at [onewheelgeek.me](https://onewheelgeek.me/posts/private-git-server-with-tailscale/).*
