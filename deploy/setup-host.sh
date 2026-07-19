#!/usr/bin/env bash
# One-shot (idempotent) setup for the owg.fyi host: docker, firewall,
# fail2ban, ssh hardening, auto-updates, swap, and a fun shell.
set -euo pipefail

C_ORANGE='\033[38;5;215m'; C_GREEN='\033[38;5;114m'; C_RESET='\033[0m'
say() { echo -e "${C_ORANGE}==>${C_RESET} $*"; }

export DEBIAN_FRONTEND=noninteractive

# ---------------------------------------------------------------- swap
if ! swapon --show | grep -q /swapfile; then
  say "adding 1G swapfile (tiny box insurance)"
  fallocate -l 1G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

# ---------------------------------------------------------------- packages
say "apt update + base packages"
apt-get update -qq
apt-get install -y -qq ufw fail2ban unattended-upgrades curl git htop vim ca-certificates >/dev/null

# ---------------------------------------------------------------- docker
if ! command -v docker >/dev/null; then
  say "installing docker (get.docker.com)"
  curl -fsSL https://get.docker.com | sh >/dev/null 2>&1
  systemctl enable --now docker
fi

# ---------------------------------------------------------------- firewall
say "configuring ufw (22, 80, 443, 1965)"
ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw allow 22/tcp comment 'ssh' >/dev/null
ufw allow 80/tcp comment 'http' >/dev/null
ufw allow 443/tcp comment 'https' >/dev/null
ufw allow 1965/tcp comment 'gemini' >/dev/null
ufw --force enable >/dev/null

# ---------------------------------------------------------------- fail2ban
say "configuring fail2ban (sshd, systemd backend)"
cat > /etc/fail2ban/jail.local <<'EOF'
[DEFAULT]
backend = systemd
bantime = 1h
findtime = 15m
maxretry = 5

[sshd]
enabled = true
EOF
systemctl enable --now fail2ban >/dev/null
systemctl restart fail2ban

# ---------------------------------------------------------------- sshd hardening
if [ -s /root/.ssh/authorized_keys ]; then
  say "hardening sshd (keys only, root via key only)"
  cat > /etc/ssh/sshd_config.d/90-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
X11Forwarding no
MaxAuthTries 4
EOF
  sshd -t && systemctl reload ssh
else
  say "SKIPPING sshd hardening: no authorized_keys found!"
fi

# ---------------------------------------------------------------- auto updates
say "enabling unattended security upgrades"
cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF

# ---------------------------------------------------------------- starship prompt
if ! command -v starship >/dev/null; then
  say "installing starship prompt"
  curl -sS https://starship.rs/install.sh | sh -s -- -y >/dev/null
fi
mkdir -p /root/.config
cat > /root/.config/starship.toml <<'EOF'
format = """
[╭─](dim white)[ owg.fyi ](bold fg:235 bg:215)[](fg:215) $directory$git_branch$docker_context$cmd_duration
[╰─](dim white)$character"""

[character]
success_symbol = "[❯](bold fg:215)"
error_symbol = "[❯](bold red)"

[directory]
style = "bold fg:114"
truncation_length = 4

[git_branch]
symbol = " "
style = "fg:208"

[docker_context]
disabled = true

[cmd_duration]
min_time = 2000
style = "dim yellow"
EOF
grep -q 'starship init' /root/.bashrc || cat >> /root/.bashrc <<'EOF'

# --- capsule niceties ---
eval "$(starship init bash)"
alias ll='ls -lah --color=auto'
alias dc='docker compose'
alias caplogs='docker logs -f capsule'
alias pplogs='docker logs -f pullpilot'
alias stack='cd /opt/owg && docker compose ps'
EOF

# ---------------------------------------------------------------- motd
say "installing MOTD"
chmod -x /etc/update-motd.d/* 2>/dev/null || true
cat > /etc/update-motd.d/01-owg <<'EOF'
#!/bin/sh
O='\033[38;5;215m'; G='\033[38;5;114m'; D='\033[2m'; R='\033[0m'
printf "${O}"
cat <<'ART'
   ██████╗ ██╗    ██╗ ██████╗    ███████╗██╗   ██╗██╗
  ██╔═══██╗██║ █╗ ██║██║  ═══╝   ██╔═══╝ ╚██╗ ██╔╝██║
  ██║   ██║██║███╗██║██║  ███╗   █████╗   ╚████╔╝ ██║
  ╚██████╔╝╚███╔███╔╝╚██████╔╝██╗██╔══╝    ╚██╔╝  ██║
   ╚═════╝  ╚══╝╚══╝  ╚═════╝ ╚═╝██║        ██║   ██║
                                 ╚═╝        ╚═╝   ╚═╝
ART
printf "${R}"
printf "  ${D}gemini://owg.fyi  ·  https://owg.fyi${R}\n\n"
printf "  ${G}uptime${R}  %s\n" "$(uptime -p | sed 's/up //')"
printf "  ${G}disk${R}    %s\n" "$(df -h / | awk 'NR==2 {print $3" / "$2" ("$5")"}')"
printf "  ${G}memory${R}  %s\n" "$(free -m | awk 'NR==2 {printf "%dM / %dM", $3, $2}')"
if command -v docker >/dev/null 2>&1; then
  UP=$(docker ps --format '{{.Names}}' 2>/dev/null | tr '\n' ' ')
  printf "  ${G}docker${R}  %s\n" "${UP:-none running}"
fi
echo
EOF
chmod +x /etc/update-motd.d/01-owg

mkdir -p /opt/owg
say "done — host is locked down and fabulous"
