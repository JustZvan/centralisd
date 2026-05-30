## Data center management for all.

### INSTALLATION

We do not provide pre-built executables and most likely never will.
A centralisd binary includes FontAwesome which may be a hassle with licesning. Just build your own, okay?

Install Go and Task on a Linux machine. Build a binary with "task build".

In the "dist" directory a "centralisd" executable will be present.
On all the servers you want to install centralisd on put it into /opt/centralis/centralisd.

#### GENERAL SETUP

Create an init service that will run centralisd as root. Example for systemd (the worst but most popular init system):
```
[Unit]
Description=Centralis
After=network.target
StartLimitIntervalSec=0
[Service]
Type=simple
Restart=always
RestartSec=1
User=root
ExecStart=/opt/centralis/centralisd

[Install]
WantedBy=multi-user.target
```

#### ORCHESTRATOR

Create an /etc/centralisd.yaml file with the following contents:

```
nodetype: "orchestrator"
orchestrator:
  weblisten: "localhost:8090"
  tcplisten: ":49150"
  dbpath: "./centralis.db"
  masterwhitelist:
    nl-amsterdam-1:
      - id: "_L1b4UewpMk_tiijNLrd3kPTh8bvHNZNBdRN2UUwH_A"
  statettlseconds: 60
```

You may tweak this as you like.
The ID is the ID of the master node. You will generate this soon.

#### MASTER

Make an /etc/centralisd.yaml file with the following contents:

```
nodetype: master
master:
  orchestrator: "<your orchestrator ip>:49150"
  cluster: "<cluster name>"
  advertise: ":49149"
  pubkeypath: "/etc/centralis/ed25519_public.pem"
  privkeypath: "/etc/centralis/ed25519_private.pem"
  allowednodes: []
```

Generate keys with OpenSSL in a root shell:

```
openssl genpkey -algorithm ed25519 -out /etc/centralis/ed25519_private.pem
openssl pkey -in /etc/centralis/ed25519_private.pem -pubout -out /etc/centralis/ed25519_public.pem
```

Run the Master node. It will say that it failed to connect to Orchestrator due to a failed handshake. Copy it's ID and put it into the masterwhitelist inside of the orchestrator's yaml file.

#### SLAVE

The Slave has some dependencies. Install Docker with their [official guide](https://docs.docker.com/engine/install/debian/). DO NOT INSTALL DOCKER.IO FROM DEBIAN REPOS. YOU MUST USE THE OFFICIAL DOCKER REPOSITORY.
Install QEMU and libvirt. On Debian that's "apt install virt-manager qemu-system-x86 qemu-utils libvirt-daemon-system".
Start libvirtd. On systemd that's "systemctl enable libvirtd --now".

Make an /etc/centralisd.yaml file with the following contents:

```
nodetype: slave

slave:
  master: <master ip>:49149
  pubkeypath: /etc/centralis/ed25519_public.pem
  privkeypath: /etc/centralis/ed25519_private.pem
```

Run the Slave. It will say that the handshake failed. Copy it's ID and add it to the allowednodes in the Master.

### Contact

Please send me an email over at justzvan@justzvan.xyz (don't forget the GPG encryption!) or shoot me a DM on Matrix at @justzvan@justzvan.xyz

