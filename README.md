# Proxy command for ssh over HTTPS proxy

`gocorkscrew` is a command that inspired by corkscrew (https://github.com/bryanpkc/corkscrew) but it supports HTTPS.

## Install

```
go get github.com/dialogbox/gocorkscrew
```

## Usage
Adds proxycommand settings to .ssh/config

```
Host host01
  User username
  HostName host01.resolvable.name.com
  ProxyCommand gocorkscrew http your.https.proxy.com 80 %h %p

Host host02
  User username
  HostName host02.resolvable.name.com
  PasswordAuthentication no
  ProxyCommand gocorkscrew https your.https.proxy.com 443 %h %p
```

Now, you can connect using ssh command like usual.

```
% ssh host01
Welcome to Ubuntu 16.04.3 LTS (GNU/Linux 4.4.0-101-generic x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/advantage

  Get cloud support with Ubuntu Advantage Cloud Guest:
    http://www.ubuntu.com/business/services/cloud

6 packages can be updated.
6 updates are security updates.


Last login: Wed Nov 22 03:19:01 2017 from x.x.x.x
username@host01:~$
```

## TODO

* Authentication
