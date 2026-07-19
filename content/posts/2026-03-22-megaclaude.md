---
title: "Megaclaude: Sandboxed Claude Code in Docker"
date: 2026-03-22
---

# Megaclaude: Sandboxed Claude Code in Docker

I love Claude Code. It's become a core part of how I build things. But there's a friction point that drives me nuts: the constant permission prompts.

*Can I create this file? Can I run this command? Can I search the web? Can I install this package?*

Yes. Yes to all of it. I gave you a task, go do it. I believe in you.

This is especially painful for greenfield projects. When I have an idea for something new and I want to quickly prototype it, iterate on it, see if the concept has legs, Claude Code is incredible. But greenfield work is where the permission prompts are at their absolute worst. Claude is constantly researching, pulling up library docs, creating new files from scratch, installing dependencies, running tests for the first time. Every single one of those actions triggers a prompt. You end up babysitting the thing, clicking "yes" every 30 seconds, which kind of defeats the purpose of having an AI assistant do the work. I feel like a drinky bird.

![Interpretation of me approving Claude Code permissions](/media/megaclaude/drinky.png)

The permission system exists for good reasons. When Claude Code has access to your actual machine, your git credentials, your SSH keys, your filesystem, you *want* guardrails. But when I'm exploring an idea and I just want Claude to go heads-down and build something, those guardrails become a leash. I want to hand it a mission, walk away, make a coffee, and come back to results.

So I built `megaclaude`. It's a bash script that locks Claude Code inside a Docker container with `--dangerously-skip-permissions` and lets it go absolutely feral in a sandbox where it can't do anything too catastrophic. Think of it as a padded room for your AI coding assistant.

## The Idea

The concept is simple:

1. Spin up a Docker container with your project directory mounted
2. Install Claude Code and [mise](https://mise.jdx.dev/) (for managing project toolchains) inside the container
3. Launch Claude with `--dangerously-skip-permissions` so it never asks for confirmation
4. Keep everything else (git credentials, SSH keys, host filesystem) *outside* the container

Claude gets free reign within the sandbox. It can install packages, create files, run tests, modify code, hit the internet for documentation. But it can't push to your repos, can't touch anything outside `/workspace`, and can't mess with your host machine. What happens in the container stays in the container. Mostly.

Is it *perfectly* safe? No, the container has internet access, which is a known trade-off. Claude needs the internet to look things up, and I'm okay with that risk. But the blast radius is contained to the project directory and a throwaway container. Claude can't accidentally `rm -rf /` your system, can't do a destructive `git push --force` to your repos, and can't poke around in your other projects. If something goes sideways, `docker rm` and you're back to clean.

## The Script

The whole thing is a single bash script. Let's walk through it.

### The Dockerfile (Generated at Runtime)

Rather than maintaining a separate Dockerfile, `megaclaude` generates one in a temp directory at build time. The whole image definition lives *inside* the bash script. Turtles all the way down. The image is based on Ubuntu 24.04 with a reasonable set of development tools:

```dockerfile
FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    file \
    git \
    gnupg \
    jq \
    less \
    libssl-dev \
    locales \
    man-db \
    openssh-client \
    pkg-config \
    python3 \
    python3-pip \
    python3-venv \
    ripgrep \
    software-properties-common \
    sudo \
    tree \
    unzip \
    vim \
    wget \
    zip \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN locale-gen en_US.UTF-8
ENV LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

RUN mkdir -p /workspace
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
```

Nothing fancy. The image is a general-purpose dev environment. The interesting stuff happens in the entrypoint.

### UID/GID Matching

One detail that matters more than you'd think: the container creates a user that matches your host UID and GID. This means files Claude creates inside `/workspace` are owned by *you* on the host, not by `root`.

```bash
DEV_UID="${DEV_UID:-1000}"
DEV_GID="${DEV_GID:-1000}"
DEV_USER="${DEV_USER:-dev}"

if ! getent group "$DEV_GID" >/dev/null 2>&1; then
    groupadd -g "$DEV_GID" "$DEV_USER" 2>/dev/null
fi

if ! id "$DEV_USER" >/dev/null 2>&1; then
    useradd -m -u "$DEV_UID" -g "$DEV_GID" -s /bin/bash "$DEV_USER" 2>/dev/null
fi
```

Without this, you'd end up with root-owned files scattered across your project directory after every session. If you've ever done the `sudo chown -R` dance after a Docker mishap, you know the pain.

### Persistent Home Volume

Claude Code and mise are installed into the user's home directory. Reinstalling them on every container start would be painful (about 30 seconds each time), so `megaclaude` uses a named Docker volume for the home directory:

```bash
HOME_VOLUME="megaclaude-home"

if ! docker volume inspect "$HOME_VOLUME" &>/dev/null; then
    docker volume create "$HOME_VOLUME" >/dev/null
    FIRST_RUN=true
else
    # Volume exists, claude + mise already cached
fi
```

First run installs everything. Subsequent runs skip straight to launching Claude. The volume persists your Claude authentication too, so you only need to log in once.

### Bootstrapping Tools

On first run, the entrypoint installs Claude Code and mise into the persistent home volume:

```bash
if [[ ! -x "$CLAUDE_BIN" ]]; then
    echo "⚡ First run - bootstrapping tools"

    echo "  [1/2] Installing Claude Code..."
    run_as_user 'curl -fsSL https://claude.ai/install.sh | bash'

    echo "  [2/2] Installing mise..."
    run_as_user 'curl -fsSL https://mise.run | sh'
fi
```

And if your project has a `mise.toml` or `.tool-versions` file, it automatically installs the project's toolchain:

```bash
if [[ -f .mise.toml ]] || [[ -f .tool-versions ]] || [[ -f mise.toml ]]; then
    $MISE_BIN trust 2>/dev/null
    $MISE_BIN install --yes 2>/dev/null
fi
```

This is one of my favourite parts. I use [mise](https://mise.jdx.dev/) for managing language runtimes across all my projects (Node versions, Go versions, Python versions, you name it), and having it as a first-class concept in `megaclaude` means Claude automatically gets access to the right toolchain for whatever project it's working on. No manual setup, no "sorry, I don't have Go installed" halfway through a task.

### The Mission Briefing

Before launching, the script prompts you for a mission. Think of it like giving a contractor a work order before locking them in the building:

```bash
step "Mission briefing"

echo "  What should Claude work on? Describe the task, goal, or"
echo "  feature you want built. Be as specific as you like."
echo "  (Empty = interactive mode, no initial prompt)"

echo "  Mission:"
IFS= read -r MISSION

if [[ -n "$MISSION" ]]; then
    echo "  Any constraints, preferences, or context? (optional)"
    IFS= read -r CONTEXT

    if [[ -n "$CONTEXT" ]]; then
        MISSION="${MISSION}

Additional context: ${CONTEXT}"
    fi
fi
```

If you provide a mission, it gets injected into a `CLAUDE.md` file in the container's home directory, which Claude Code reads automatically on startup:

```bash
if [[ -n "${MEGA_MISSION:-}" ]]; then
    mkdir -p "$HOME_DIR/.claude"
    cat > "$HOME_DIR/.claude/CLAUDE.md" <<MISSION_EOF
# MEGACLAUDE SESSION

You are running in a fully sandboxed Docker container with --dangerously-skip-permissions.
You have complete freedom to read, write, execute, and install anything — nothing can break.
Be bold and autonomous. Do not ask for permission or confirmation — just do the work.

## Your Mission

${MEGA_MISSION}

---
Work autonomously until the mission is fully complete. Show your work as you go.
MISSION_EOF
fi
```

This is the key trick. The `CLAUDE.md` file tells Claude it's in a sandbox and should act accordingly: no hesitation, no asking for permission, just execute. Combined with `--dangerously-skip-permissions`, Claude just... goes. No more drinky bird.

### Launching the Container

Finally, everything comes together in the `docker run`:

```bash
exec docker run -it --rm \
    --name "$CONTAINER_NAME" \
    -v "${PROJECT_DIR}:/workspace" \
    -v "${HOME_VOLUME}:/home/${HOST_USER}" \
    -e "DEV_UID=${HOST_UID}" \
    -e "DEV_GID=${HOST_GID}" \
    -e "DEV_USER=${HOST_USER}" \
    -e "MEGA_MISSION=${MISSION}" \
    -e "TERM=${TERM:-xterm-256color}" \
    -e "LANG=en_US.UTF-8" \
    --hostname "megaclaude" \
    "${IMAGE_NAME}:${IMAGE_TAG}" \
    $SHELL_ARG
```

Worth calling out:

- **`-v "${PROJECT_DIR}:/workspace"`** binds your project in. Changes Claude makes are real and persist after the container exits. This is the one place where the sandbox touches your host.
- **`-v "${HOME_VOLUME}:/home/${HOST_USER}"`** is the persistent volume for Claude, mise, and auth.
- **`--rm`** makes the container disposable. It cleans up after itself.
- **No git credentials.** Your `~/.gitconfig`, `~/.ssh`, and any credential helpers stay on the host. Claude can *use* git inside the container (for diffing, branching, etc.), but it can't push anywhere. All commit and no push makes Claude a safe boy.

## Usage

It's straightforward:

```bash
# Launch Claude in a project
megaclaude ./my-project

# Full rebuild (nuke cached tools)
megaclaude ./my-project --rebuild

# Drop into a shell to poke around
megaclaude ./my-project --shell
```

A typical workflow looks like:

1. `megaclaude ./my-project`
2. Type in a mission: "Add unit tests for the authentication module"
3. Walk away
4. Come back, review the changes with `git diff`
5. Commit what you like, discard what you don't

## The Trade-offs

I'm not going to pretend this is perfect.

**What it gives you:**
- Claude Code with zero permission friction
- A sandbox that limits blast radius to your project directory
- Persistent tooling across sessions (no re-installing every time)
- Automatic project toolchain setup via mise
- A mission-based workflow where you describe what you want and let Claude execute

**What it doesn't give you:**
- Network isolation. The container has full internet access. Claude needs this to search docs and install packages, so the sandbox isn't hermetically sealed. It's a screen door, not a vault.
- Git push protection *if you break it*. Claude can't push (no credentials), but if you *were* to mount your SSH keys or gitconfig, that protection disappears. Don't do that. Resist the temptation.
- Infinite trust. Claude is autonomous inside the container, but it's still an LLM. Review the output. Use `git diff`. Trust but verify. Mostly verify.

## Why Not Just Use `--dangerously-skip-permissions` Directly?

You could! But then Claude has access to your *entire machine*. It can read your SSH keys, your environment variables, your other projects, your credentials. The Docker sandbox keeps the worst case to "it makes a mess of one project directory." That's a much smaller kaboom.

It's the difference between giving someone the keys to one room versus the whole building. And in this room, the walls are padded and the windows don't open.

## Wrapping Up

`megaclaude` has become my go-to for "fire and forget" Claude Code sessions. Type a mission, go make lunch, come back to a completed task (or at least a solid first pass). It's not replacing my normal interactive Claude Code sessions, those are great when I want to be hands-on, but for autonomous work the sandboxed approach just makes sense.

The whole thing is ~420 lines of bash. No external dependencies beyond Docker. It builds its own image, manages its own volumes, and gets out of your way. The full script is here if you want to grab it:

[View the full script on GitHub Gist](https://gist.github.com/jclement/7a02d8f1979a1c37085bd3ecc3fceb17)

---

*Originally published at [onewheelgeek.me](https://onewheelgeek.me/posts/megaclaude/).*
