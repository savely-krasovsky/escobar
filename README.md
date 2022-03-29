# Escobar
[![Go](https://github.com/L11R/escobar/actions/workflows/go.yml/badge.svg)](https://github.com/L11R/escobar/actions/workflows/go.yml)

This is an alternative to `cntlm` utility, but written in Go. It's aim to be simpler and easier to customize
for own needs. Mainly tested against McAfee Web Gateway. Tested in Linux, macOS and Windows, but should work on other
OS like Android.

Unlike `cntlm` it uses Kerberos-based authorization. It also supports Basic Authorization (as dedicated mode),
this is useful while KDC is unavailable (e.g. while using VPN).

As extra feature it deploys small static server with two routes:
1. `GET /proxy.pac` — simple PAC-file (Proxy Auto-Configuration).
2. `GET /ca.crt` — always actual root certificate. Useful during first setup to retrieve Man-In-The-Middle root
certificate (corporate proxy in our case) and add it as trusted.

### Compiling
Escobar is usual Go-application. You need only Go compiler downloaded from [official site](https://golang.org/).
```bash
go build -o escobar cmd/escobar/main.go
```

Of course you can use cross-compiling:
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o escobar cmd/escobar/main.go
# macOS
GOOS=darwin GOARCH=amd64 go build -o escobar cmd/escobar/main.go
# Windows
GOOS=windows GOARCH=amd64 go build -o escobar.exe cmd/escobar/main.go
# ARM-based Android
GOOS=android GOARCH=arm64 go build -o escobar cmd/escobar/main.go
```

### Testing
```bash
cd ~/projects/escobar
go test ./...
```
You need to turn off inlining to test it in Windows:
```bash
go test -gcflags=-l ./...
```

### As service
You can also (and most possibly will) turn utility into service. For example in Linux you could use `systemd`.
Basic service config:
```
[Unit]
Description=Escobar
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=escobar
Group=escobar
EnvironmentFile=-/etc/default/escobar
WorkingDirectory=/opt/escobar
ExecStart=/opt/escobar/escobar
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

In this case utility will use environment variable from `/etc/default/escobar` file.
You can check them out in `configs` package.

In macOS you can use `launchd`. Check details in the 
[official documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html).

### Status
This app provides as is and proper work could not be guaranteed.
You are free to contribute by creating issues and PR.

### Using
Run `escobar --help` to get this detailed help:
```
Usage:
  escobar [OPTIONS]

Application Options:
  -v, --verbose                                                   Verbose logs [$ESCOBAR_VERBOSE]
  -V, --version                                                   Escobar version

Proxy args:
  -a, --proxy.addr=                                               Proxy address (default: localhost:3128) [$ESCOBAR_PROXY_ADDR]
  -d, --proxy.downstream-proxy-url=http://proxy.evil.corp:9090    Downstream proxy URL [$ESCOBAR_PROXY_DOWNSTREAM_PROXY_URL]
      --proxy.ping-url=                                           URL to ping anc check credentials validity (default: https://www.google.com/) [$ESCOBAR_PROXY_PING_URL]
  -b, --proxy.basic-mode                                          Basic authorization mode (do not use if Kerberos works for you) [$ESCOBAR_PROXY_BASIC_MODE]
  -m, --proxy.manual-mode                                         Turns off Windows SSPI (which is enabled by default) [$ESCOBAR_PROXY_MANUAL_MODE]

Downstream Proxy authentication:
  -u, --proxy.downstream-proxy-auth.user=                         Downstream Proxy user [$ESCOBAR_PROXY_DOWNSTREAM_PROXY_AUTH_USER]
  -p, --proxy.downstream-proxy-auth.password=                     Downstream Proxy password [$ESCOBAR_PROXY_DOWNSTREAM_PROXY_AUTH_PASSWORD]
  -k, --proxy.downstream-proxy-auth.keytab=                       Downstream Proxy path to keytab-file [$ESCOBAR_PROXY_DOWNSTREAM_PROXY_AUTH_KEYTAB]

Kerberos options:
      --proxy.kerberos.realm=EVIL.CORP                            Kerberos realm [$ESCOBAR_PROXY_KERBEROS_REALM]
      --proxy.kerberos.kdc=kdc.evil.corp:88                       Key Distribution Center (KDC) address [$ESCOBAR_PROXY_KERBEROS_KDC_ADDR]
      --proxy.kerberos.kpasswd-server=kpasswd.evil.corp:464       Server address where all the password changes are performed [$ESCOBAR_PROXY_KERBEROS_KPASSWD_SERVER_ADDR]

Server timeouts:
      --proxy.timeouts.server.read=                               HTTP server read timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_SERVER_READ]
      --proxy.timeouts.server.read-header=                        HTTP server read header timeout (default: 30s) [$ESCOBAR_PROXY_TIMEOUTS_SERVER_READ_HEADER]
      --proxy.timeouts.server.write=                              HTTP server write timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_SERVER_WRITE]
      --proxy.timeouts.server.idle=                               HTTP server idle timeout (default: 1m) [$ESCOBAR_PROXY_TIMEOUTS_SERVER_IDLE]

Client timeouts:
      --proxy.timeouts.client.read=                               Client read timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_CLIENT_READ]
      --proxy.timeouts.client.write=                              Client write timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_CLIENT_WRITE]
      --proxy.timeouts.client.keepalive-period=                   Client keepalive period (default: 1m) [$ESCOBAR_PROXY_TIMEOUTS_CLIENT_KEEPALIVE_PERIOD]

Downstream Proxy timeouts:
      --proxy.timeouts.downstream.dial=                           Downstream proxy dial timeout (default: 10s) [$ESCOBAR_PROXY_TIMEOUTS_DOWNSTREAM_DIAL]
      --proxy.timeouts.downstream.read=                           Downstream proxy read timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_DOWNSTREAM_READ]
      --proxy.timeouts.downstream.write=                          Downstream proxy write timeout (default: 0s) [$ESCOBAR_PROXY_TIMEOUTS_DOWNSTREAM_WRITE]
      --proxy.timeouts.downstream.keepalive-period=               Downstream proxy keepalive period (default: 1m) [$ESCOBAR_PROXY_TIMEOUTS_DOWNSTREAM_KEEPALIVE_PERIOD]

Static args:
      --static.addr=                                              Static server address (default: localhost:3129) [$ESCOBAR_STATIC_ADDR]

Help Options:
  -h, --help                                                      Show this help message
```

### Keytab-file support
I could also suggest to use keytab-files instead of passing plain password. It's safer and less accessible.
Below you can read about recommended setup.

1. Create service user `escobar` and lock it:

    ```bash
    # useradd -M escobar
    # usermod -L escobar
    ```
2. Create `escobar` directory inside `/etc` and give `escobar` user proper rights:

    ```bash
    # mkdir /etc/escobar
    # chown escobar:escobar /etc/escobar
    # chmod 0700 /etc/escobar
    ```
3. Create proper keytab-file by using `ktutil` utility:

    ```bash
    $ sudo -u escobar ktutil
    ktutil: add_entry -password -p ivanovii@EVIL.CORP -k 0 -e aes256-cts-hmac-sha1-96
    Password for ivanovii@EVIL.CORP:
    ktutil: write_kt /etc/escobar/ivanovii.keytab
    ktutil: exit
    ```

#### Keytab-file troubleshooting
* Check rights:
    * Directory should have `0700`.
    * Keytab-file itself `0600`.
    * Both directory and keytab should be owned by user who execute software.
* Check principal name. Usually it's your username and realm: `username@REALM.COM`
* Check KVNO, it could be invalid. Everytime you change your password, KVNO updates by rule `Current KVNO + 1`.
* Check encryption type. In example guide above I showed how to generate keytab-file with `aes256-cts-hmac-sha1-96`.
But in your case it could be another cipher.

In case of principal name, KVNO and encryption type program should print what does it looking for.
